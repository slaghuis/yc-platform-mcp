package spec

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/index"
	"github.com/pb33f/libopenapi/orderedmap"
	"gopkg.in/yaml.v3"
)

// Load reads one OpenAPI file, or every .json, .yaml, and .yml file in a
// directory, and returns the resolved model. path is a file or a directory.
// JSON and YAML are chosen from the file extension. $ref cycles are kept in
// the model; other parse errors fail the load.
func Load(path string) (*SpecStore, error) {
	files, err := specFiles(path)
	if err != nil {
		return nil, err
	}
	store := &SpecStore{byName: map[string]int{}}
	for _, file := range files {
		svc, err := loadFile(file)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		key := strings.ToLower(svc.Name)
		if _, exists := store.byName[key]; exists {
			return nil, fmt.Errorf("%s: duplicate service %q", file, svc.Name)
		}
		store.byName[key] = len(store.services)
		store.services = append(store.services, svc)
	}
	sort.Slice(store.services, func(i, j int) bool {
		return store.services[i].Name < store.services[j].Name
	})
	store.byName = make(map[string]int, len(store.services))
	for i := range store.services {
		store.byName[strings.ToLower(store.services[i].Name)] = i
	}
	return store, nil
}

func specFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat spec path: %w", err)
	}
	if !info.IsDir() {
		if !isSpecExt(path) {
			return nil, fmt.Errorf("unsupported spec extension %q (want .json, .yaml, or .yml)", filepath.Ext(path))
		}
		return []string{path}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read spec directory: %w", err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !isSpecExt(entry.Name()) {
			continue
		}
		files = append(files, filepath.Join(path, entry.Name()))
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no OpenAPI files (.json, .yaml, .yml) in %s", path)
	}
	return files, nil
}

func isSpecExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func loadFile(path string) (Service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Service{}, fmt.Errorf("read spec: %w", err)
	}
	if err := validateEncoding(path, data); err != nil {
		return Service{}, err
	}
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		return Service{}, fmt.Errorf("parse OpenAPI: %w", err)
	}
	model, err := doc.BuildV3Model()
	if model == nil {
		if err == nil {
			err = errors.New("empty model")
		}
		return Service{}, fmt.Errorf("build OpenAPI model: %w", err)
	}
	if fatal := fatalBuildErrors(err); fatal != nil {
		return Service{}, fmt.Errorf("build OpenAPI model: %w", fatal)
	}
	return serviceFromModel(serviceName(path), &model.Model), nil
}

func validateEncoding(path string, data []byte) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if !json.Valid(data) {
			return fmt.Errorf("invalid JSON in %s", filepath.Base(path))
		}
	case ".yaml", ".yml":
		var v any
		if err := yaml.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("invalid YAML in %s: %w", filepath.Base(path), err)
		}
	default:
		return fmt.Errorf("unsupported spec extension %q", filepath.Ext(path))
	}
	return nil
}

// fatalBuildErrors drops circular-reference diagnostics. libopenapi still
// returns a usable model for those, and the schema walker stops the cycle.
func fatalBuildErrors(err error) error {
	var fatal []error
	for _, e := range flattenErrors(err) {
		var refErr *index.ResolvingError
		if errors.As(e, &refErr) && refErr.CircularReference != nil {
			continue
		}
		fatal = append(fatal, e)
	}
	return errors.Join(fatal...)
}

func flattenErrors(err error) []error {
	if err == nil {
		return nil
	}
	type joined interface{ Unwrap() []error }
	if u, ok := err.(joined); ok {
		var out []error
		for _, inner := range u.Unwrap() {
			out = append(out, flattenErrors(inner)...)
		}
		if len(out) > 0 {
			return out
		}
	}
	return []error{err}
}

func serviceName(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	switch strings.ToLower(name) {
	case "openapi", "swagger":
		parent := filepath.Base(filepath.Dir(path))
		if parent != "." && parent != string(filepath.Separator) {
			return parent
		}
	}
	return name
}

func serviceFromModel(name string, doc *v3.Document) Service {
	svc := Service{
		Name:    name,
		Title:   name,
		Tags:    []Tag{},
		OpenAPI: doc.Version,
	}
	if doc.Info != nil {
		if title := strings.TrimSpace(doc.Info.Title); title != "" {
			svc.Title = title
		}
		svc.Version = strings.TrimSpace(doc.Info.Version)
		svc.Description = strings.TrimSpace(doc.Info.Description)
	}
	for _, server := range doc.Servers {
		if server == nil {
			continue
		}
		if url := serverURL(server); url != "" {
			svc.Servers = append(svc.Servers, url)
		}
	}
	if len(svc.Servers) > 0 {
		svc.BaseURL = svc.Servers[0]
	}
	svc.Security = securitySchemes(doc.Components)
	svc.GlobalAuth = securityRequirements(doc.Security)
	svc.Schemas = componentSchemas(name, doc.Components)
	svc.Endpoints = endpoints(name, doc, svc.GlobalAuth)
	svc.Tags = tags(doc, svc.Endpoints)
	return svc
}

func serverURL(server *v3.Server) string {
	url := server.URL
	eachPair(server.Variables, func(name string, variable *v3.ServerVariable) {
		if variable == nil {
			return
		}
		url = strings.ReplaceAll(url, "{"+name+"}", variable.Default)
	})
	return url
}

func securitySchemes(components *v3.Components) []SecurityScheme {
	if components == nil || components.SecuritySchemes == nil {
		return nil
	}
	var out []SecurityScheme
	eachPair(components.SecuritySchemes, func(name string, scheme *v3.SecurityScheme) {
		if scheme == nil {
			return
		}
		out = append(out, SecurityScheme{
			Name:             name,
			Type:             scheme.Type,
			Description:      strings.TrimSpace(scheme.Description),
			In:               scheme.In,
			ParameterName:    scheme.Name,
			Scheme:           scheme.Scheme,
			BearerFormat:     scheme.BearerFormat,
			OpenIDConnectURL: scheme.OpenIdConnectUrl,
		})
	})
	return out
}

func securityRequirements(reqs []*base.SecurityRequirement) []SecurityRequirement {
	if reqs == nil {
		return nil
	}
	out := make([]SecurityRequirement, 0, len(reqs))
	for _, req := range reqs {
		if req == nil {
			continue
		}
		item := SecurityRequirement{}
		eachPair(req.Requirements, func(name string, scopes []string) {
			item.Schemes = append(item.Schemes, SecuritySchemeUse{
				Name:   name,
				Scopes: copyStrings(scopes),
			})
		})
		out = append(out, item)
	}
	return out
}

func componentSchemas(service string, components *v3.Components) []SchemaDefinition {
	if components == nil || components.Schemas == nil {
		return nil
	}
	var out []SchemaDefinition
	eachPair(components.Schemas, func(name string, proxy *base.SchemaProxy) {
		out = append(out, buildSchema(service, name, proxy))
	})
	return out
}

func tags(doc *v3.Document, endpoints []Endpoint) []Tag {
	descriptions := map[string]string{}
	var order []string
	seen := map[string]bool{}
	for _, tag := range doc.Tags {
		if tag == nil || tag.Name == "" || seen[tag.Name] {
			continue
		}
		seen[tag.Name] = true
		order = append(order, tag.Name)
		descriptions[tag.Name] = strings.TrimSpace(tag.Description)
	}
	for _, ep := range endpoints {
		for _, name := range ep.Tags {
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			order = append(order, name)
		}
	}
	out := make([]Tag, 0, len(order))
	for _, name := range order {
		out = append(out, Tag{Name: name, Description: descriptions[name]})
	}
	return out
}

func endpoints(service string, doc *v3.Document, global []SecurityRequirement) []Endpoint {
	if doc.Paths == nil || doc.Paths.PathItems == nil {
		return nil
	}
	var out []Endpoint
	eachPair(doc.Paths.PathItems, func(path string, item *v3.PathItem) {
		if item == nil {
			return
		}
		ops := item.GetOperations()
		eachPair(ops, func(method string, op *v3.Operation) {
			if op == nil {
				return
			}
			out = append(out, endpointFrom(service, path, strings.ToUpper(method), item, op, global))
		})
	})
	return out
}

func endpointFrom(service, path, method string, item *v3.PathItem, op *v3.Operation, global []SecurityRequirement) Endpoint {
	ep := Endpoint{
		Service:     service,
		Method:      method,
		Path:        path,
		OperationID: op.OperationId,
		Summary:     strings.TrimSpace(op.Summary),
		Description: strings.TrimSpace(op.Description),
		Tags:        copyStrings(op.Tags),
		Deprecated:  op.Deprecated != nil && *op.Deprecated,
		Parameters:  parameters(mergeParameters(item, op)),
		RequestBody: requestBody(op.RequestBody),
		Responses:   responses(op.Responses),
	}
	switch {
	case op.Security == nil:
		ep.Security = global
		if len(global) > 0 {
			ep.SecuritySource = "global"
		} else {
			ep.SecuritySource = "none"
		}
	default:
		ep.Security = securityRequirements(op.Security)
		if len(ep.Security) == 0 {
			ep.SecuritySource = "none"
		} else {
			ep.SecuritySource = "operation"
		}
	}
	return ep
}

func mergeParameters(item *v3.PathItem, op *v3.Operation) []*v3.Parameter {
	var all []*v3.Parameter
	if item != nil {
		all = append(all, item.Parameters...)
	}
	if op != nil {
		all = append(all, op.Parameters...)
	}
	type key struct{ name, in string }
	indexByKey := map[key]int{}
	var out []*v3.Parameter
	for _, param := range all {
		if param == nil || param.Name == "" {
			continue
		}
		k := key{param.Name, param.In}
		if i, ok := indexByKey[k]; ok {
			out[i] = param
			continue
		}
		indexByKey[k] = len(out)
		out = append(out, param)
	}
	return out
}

func parameters(params []*v3.Parameter) []Parameter {
	if len(params) == 0 {
		return nil
	}
	out := make([]Parameter, 0, len(params))
	for _, param := range params {
		required := strings.EqualFold(param.In, "path") || (param.Required != nil && *param.Required)
		out = append(out, Parameter{
			Name:        param.Name,
			In:          param.In,
			Description: strings.TrimSpace(param.Description),
			Required:    required,
			Deprecated:  param.Deprecated,
			Schema:      parameterSchema(param),
		})
	}
	return out
}

func parameterSchema(param *v3.Parameter) SchemaDefinition {
	if param.Schema != nil {
		return walkSchema(param.Schema, map[string]bool{}, 0)
	}
	var found SchemaDefinition
	eachPair(param.Content, func(contentType string, media *v3.MediaType) {
		if found.Type != "" || found.Ref != "" || media == nil || media.Schema == nil {
			return
		}
		found = walkSchema(media.Schema, map[string]bool{}, 0)
		if found.Description == "" {
			found.Description = contentType
		}
	})
	return found
}

func requestBody(body *v3.RequestBody) *RequestBody {
	if body == nil {
		return nil
	}
	contentType, media := preferredMedia(body.Content)
	out := &RequestBody{
		Required:    body.Required != nil && *body.Required,
		Description: strings.TrimSpace(body.Description),
		ContentType: contentType,
	}
	if media != nil && media.Schema != nil {
		out.Schema = walkSchema(media.Schema, map[string]bool{}, 0)
	}
	return out
}

func responses(resps *v3.Responses) []Response {
	if resps == nil {
		return nil
	}
	var out []Response
	eachPair(resps.Codes, func(status string, resp *v3.Response) {
		if resp == nil {
			return
		}
		out = append(out, responseFrom(status, resp))
	})
	if resps.Default != nil {
		out = append(out, responseFrom("default", resps.Default))
	}
	return out
}

func responseFrom(status string, resp *v3.Response) Response {
	contentType, media := preferredMedia(resp.Content)
	out := Response{
		Status:      status,
		Description: strings.TrimSpace(resp.Description),
		ContentType: contentType,
	}
	if media != nil && media.Schema != nil {
		out.Schema = walkSchema(media.Schema, map[string]bool{}, 0)
	}
	if out.Schema.Ref == "" && resp.Reference != "" {
		out.Schema.Ref = resp.Reference
	}
	return out
}

func preferredMedia(content *orderedmap.Map[string, *v3.MediaType]) (string, *v3.MediaType) {
	if content == nil {
		return "", nil
	}
	for _, contentType := range []string{
		"application/json",
		"application/problem+json",
		"application/merge-patch+json",
	} {
		if media := mediaByType(content, contentType); media != nil {
			return contentType, media
		}
	}
	for contentType, media := range content.FromOldest() {
		return contentType, media
	}
	return "", nil
}

func mediaByType(content *orderedmap.Map[string, *v3.MediaType], contentType string) *v3.MediaType {
	var found *v3.MediaType
	eachPair(content, func(key string, media *v3.MediaType) {
		if found == nil && key == contentType {
			found = media
		}
	})
	return found
}
