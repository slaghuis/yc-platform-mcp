package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/slaghuis/yc-platform-mcp/internal/spec"
)

const assumedBaseURL = "https://api.example.com"

// GetExampleRequestInput selects an operation and an example format.
type GetExampleRequestInput struct {
	Path    string `json:"path" jsonschema:"OpenAPI path, such as /products/{id}"`
	Method  string `json:"method" jsonschema:"HTTP method, such as GET or POST"`
	Format  string `json:"format,omitempty" jsonschema:"curl, go, or javascript; defaults to curl"`
	Service string `json:"service,omitempty" jsonschema:"microservice name, required when the path exists on more than one service"`
}

// ExampleRequest is a generated HTTP example.
type ExampleRequest struct {
	Service        string `json:"service"`
	Method         string `json:"method"`
	Path           string `json:"path"`
	URL            string `json:"url"`
	Format         string `json:"format"`
	ContentType    string `json:"content_type,omitempty"`
	Example        string `json:"example"`
	BaseURLAssumed bool   `json:"base_url_assumed,omitempty"`
}

// GetExampleRequest renders a curl, Go, or JavaScript request for an operation.
func (h *Handler) GetExampleRequest(ctx context.Context, _ *mcp.CallToolRequest, in GetExampleRequestInput) (*mcp.CallToolResult, ExampleRequest, error) {
	if err := ctx.Err(); err != nil {
		return nil, ExampleRequest{}, err
	}
	method, err := normalizeMethod(in.Method)
	if err != nil {
		return nil, ExampleRequest{}, err
	}
	if strings.TrimSpace(in.Path) == "" {
		return nil, ExampleRequest{}, fmt.Errorf("path is required")
	}
	format, err := normalizeFormat(in.Format)
	if err != nil {
		return nil, ExampleRequest{}, err
	}
	endpoint, err := h.Store.Endpoint(in.Service, in.Path, method)
	if err != nil {
		return nil, ExampleRequest{}, err
	}
	svc, ok := h.Store.Lookup(endpoint.Service)
	if !ok {
		return nil, ExampleRequest{}, fmt.Errorf("service %s not found", endpoint.Service)
	}
	example, err := renderExample(svc, endpoint, format)
	if err != nil {
		return nil, ExampleRequest{}, err
	}
	return nil, example, nil
}

func normalizeFormat(format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "curl":
		return "curl", nil
	case "go", "golang":
		return "go", nil
	case "javascript", "js", "node":
		return "javascript", nil
	default:
		return "", fmt.Errorf("unsupported format %q (want curl, go, or javascript)", format)
	}
}

func renderExample(svc spec.Service, ep spec.Endpoint, format string) (ExampleRequest, error) {
	base := svc.BaseURL
	assumed := false
	if base == "" {
		base = assumedBaseURL
		assumed = true
	}
	path := fillPath(ep.Path, ep.Parameters)
	rawURL := strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ExampleRequest{}, fmt.Errorf("build example url: %w", err)
	}
	query := parsed.Query()
	headers := map[string]string{}
	applySecurity(&svc, ep, headers, query)
	for _, param := range ep.Parameters {
		if param.In == "query" && param.Required {
			query.Set(param.Name, exampleScalar(param.Schema, param.Name))
		}
		if param.In == "header" && param.Required {
			headers[param.Name] = exampleScalar(param.Schema, param.Name)
		}
	}
	parsed.RawQuery = query.Encode()

	var body string
	contentType := ""
	if ep.RequestBody != nil {
		contentType = ep.RequestBody.ContentType
		if contentType == "" {
			contentType = "application/json"
		}
		headers["Content-Type"] = contentType
		encoded, err := json.MarshalIndent(exampleValue(ep.RequestBody.Schema), "", "  ")
		if err != nil {
			return ExampleRequest{}, fmt.Errorf("encode example body: %w", err)
		}
		body = string(encoded)
	}

	out := ExampleRequest{
		Service:        ep.Service,
		Method:         ep.Method,
		Path:           ep.Path,
		URL:            parsed.String(),
		Format:         format,
		ContentType:    contentType,
		BaseURLAssumed: assumed,
	}
	switch format {
	case "curl":
		out.Example = renderCurl(ep.Method, parsed.String(), headers, body)
	case "go":
		out.Example = renderGo(ep.Method, parsed.String(), headers, body)
	case "javascript":
		out.Example = renderJavaScript(ep.Method, parsed.String(), headers, body)
	}
	return out, nil
}

func applySecurity(svc *spec.Service, ep spec.Endpoint, headers map[string]string, query url.Values) {
	if len(ep.Security) == 0 || len(ep.Security[0].Schemes) == 0 {
		return
	}
	schemes := map[string]spec.SecurityScheme{}
	for _, scheme := range svc.Security {
		schemes[scheme.Name] = scheme
	}
	for _, use := range ep.Security[0].Schemes {
		scheme, ok := schemes[use.Name]
		if !ok {
			continue
		}
		switch strings.ToLower(scheme.Type) {
		case "http":
			if strings.EqualFold(scheme.Scheme, "basic") {
				headers["Authorization"] = "Basic <base64-user:password>"
			} else {
				headers["Authorization"] = "Bearer <token>"
			}
		case "apikey":
			name := scheme.ParameterName
			if name == "" {
				name = scheme.Name
			}
			switch strings.ToLower(scheme.In) {
			case "query":
				query.Set(name, "<api-key>")
			case "cookie":
				headers["Cookie"] = name + "=<api-key>"
			default:
				headers[name] = "<api-key>"
			}
		default:
			headers["Authorization"] = "Bearer <token>"
		}
	}
}

func fillPath(path string, params []spec.Parameter) string {
	for _, param := range params {
		if param.In != "path" {
			continue
		}
		path = strings.ReplaceAll(path, "{"+param.Name+"}", exampleScalar(param.Schema, param.Name))
	}
	return path
}

func exampleValue(schema spec.SchemaDefinition) any {
	if len(schema.Enum) > 0 {
		return schema.Enum[0]
	}
	if len(schema.OneOf) > 0 {
		return exampleValue(schema.OneOf[0])
	}
	if len(schema.AnyOf) > 0 {
		return exampleValue(schema.AnyOf[0])
	}
	if len(schema.AllOf) > 0 && len(schema.Properties) == 0 && schema.Type == "" {
		merged := schema
		for _, part := range schema.AllOf {
			merged.Properties = append(merged.Properties, part.Properties...)
			if merged.Type == "" {
				merged.Type = part.Type
			}
		}
		return exampleValue(merged)
	}
	switch strings.ToLower(schema.Type) {
	case "object", "":
		if len(schema.Properties) == 0 && schema.Items == nil {
			if schema.Type == "" {
				return "example"
			}
			return map[string]any{}
		}
		if schema.Items != nil && !strings.EqualFold(schema.Type, "object") {
			return []any{exampleValue(*schema.Items)}
		}
		obj := map[string]any{}
		for _, prop := range schema.Properties {
			obj[prop.Name] = exampleValue(prop.Schema)
		}
		return obj
	case "array":
		if schema.Items == nil {
			return []any{}
		}
		return []any{exampleValue(*schema.Items)}
	case "integer":
		return 1
	case "number":
		return 1.0
	case "boolean":
		return true
	default:
		return exampleScalar(schema, "value")
	}
}

func exampleScalar(schema spec.SchemaDefinition, name string) string {
	if len(schema.Enum) > 0 {
		return schema.Enum[0]
	}
	switch strings.ToLower(schema.Format) {
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "date-time":
		return "2026-01-01T00:00:00Z"
	case "date":
		return "2026-01-01"
	case "email":
		return "user@example.com"
	case "uri", "url":
		return "https://example.com"
	}
	switch strings.ToLower(schema.Type) {
	case "integer", "number":
		return "1"
	case "boolean":
		return "true"
	default:
		if name == "" {
			return "example"
		}
		return "example"
	}
}

func renderCurl(method, rawURL string, headers map[string]string, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "curl -X %s '%s'", method, escapeSingle(rawURL))
	for _, key := range sortedKeys(headers) {
		fmt.Fprintf(&b, " \\\n  -H '%s: %s'", key, escapeSingle(headers[key]))
	}
	if body != "" {
		fmt.Fprintf(&b, " \\\n  --data '%s'", escapeSingle(body))
	}
	return b.String()
}

func renderGo(method, rawURL string, headers map[string]string, body string) string {
	var b strings.Builder
	b.WriteString("req, err := http.NewRequest(")
	fmt.Fprintf(&b, "http.Method%s, %q", titleMethod(method), rawURL)
	if body == "" {
		b.WriteString(", nil)")
	} else {
		fmt.Fprintf(&b, ", bytes.NewReader([]byte(%q)))", body)
	}
	for _, key := range sortedKeys(headers) {
		fmt.Fprintf(&b, "\nreq.Header.Set(%q, %q)", key, headers[key])
	}
	b.WriteString("\nresp, err := http.DefaultClient.Do(req)")
	return b.String()
}

func renderJavaScript(method, rawURL string, headers map[string]string, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "await fetch(%q, {\n  method: %q", rawURL, method)
	if len(headers) > 0 {
		b.WriteString(",\n  headers: {")
		first := true
		for _, key := range sortedKeys(headers) {
			if !first {
				b.WriteString(",")
			}
			first = false
			fmt.Fprintf(&b, "\n    %q: %q", key, headers[key])
		}
		b.WriteString("\n  }")
	}
	if body != "" {
		fmt.Fprintf(&b, ",\n  body: %s", body)
	}
	b.WriteString("\n})")
	return b.String()
}

func titleMethod(method string) string {
	if method == "" {
		return ""
	}
	return strings.ToUpper(method[:1]) + strings.ToLower(method[1:])
}

func escapeSingle(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

func sortedKeys(headers map[string]string) []string {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
