package spec

import (
	"fmt"
	"strconv"
	"strings"
)

// Lookup returns one microservice by file name or title match on the file name.
// The name comparison is case-insensitive.
func (s *SpecStore) Lookup(name string) (Service, bool) {
	i, ok := s.byName[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return Service{}, false
	}
	return s.services[i], true
}

// Services returns one summary per loaded microservice, ordered by name.
func (s *SpecStore) Services() []ServiceSummary {
	out := make([]ServiceSummary, 0, len(s.services))
	for i := range s.services {
		out = append(out, s.services[i].summary())
	}
	return out
}

// Overview returns the platform summary. A single document uses that
// document's title, version, description, and base URL. Several documents
// are reported together as yc-platform.
func (s *SpecStore) Overview() Overview {
	overview := Overview{
		Title:    "yc-platform",
		Services: s.Services(),
	}
	urls := map[string]struct{}{}
	for i := range s.services {
		svc := &s.services[i]
		overview.EndpointCount += len(svc.Endpoints)
		overview.SchemaCount += len(svc.Schemas)
		if svc.BaseURL != "" {
			urls[svc.BaseURL] = struct{}{}
		}
	}
	switch len(s.services) {
	case 0:
		overview.Description = "No OpenAPI documents are loaded."
	case 1:
		svc := &s.services[0]
		overview.Title = svc.Title
		overview.Version = svc.Version
		overview.Description = svc.Description
		overview.BaseURL = svc.BaseURL
	default:
		if len(urls) == 1 {
			for url := range urls {
				overview.BaseURL = url
			}
		}
		overview.Description = fmt.Sprintf(
			"yc-platform exposes %d microservice OpenAPI documents.",
			len(s.services),
		)
	}
	return overview
}

// Search returns operations whose path, method, summary, description,
// operationId, or tags contain keyword. service filters by microservice name
// or OpenAPI tag when it is not empty. keyword and service are case-insensitive.
func (s *SpecStore) Search(keyword, service string) []EndpointSummary {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	var out []EndpointSummary
	for i := range s.services {
		svc := &s.services[i]
		for _, ep := range svc.Endpoints {
			if service != "" && !matchesService(svc, ep, service) {
				continue
			}
			if keyword != "" && !matchesKeyword(ep, keyword) {
				continue
			}
			out = append(out, ep.Brief())
		}
	}
	if out == nil {
		out = []EndpointSummary{}
	}
	return out
}

// Endpoint returns the operation for path and method. service disambiguates
// when more than one document defines that operation. method is case-insensitive.
func (s *SpecStore) Endpoint(service, path, method string) (Endpoint, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	var hits []Endpoint
	for i := range s.services {
		svc := &s.services[i]
		if service != "" && !strings.EqualFold(svc.Name, service) {
			continue
		}
		for _, ep := range svc.Endpoints {
			if ep.Path == path && ep.Method == method {
				hits = append(hits, ep)
			}
		}
	}
	switch len(hits) {
	case 0:
		if service != "" {
			return Endpoint{}, fmt.Errorf("endpoint %s %s not found on service %s", method, path, service)
		}
		return Endpoint{}, fmt.Errorf("endpoint %s %s not found", method, path)
	case 1:
		return hits[0], nil
	default:
		names := make([]string, len(hits))
		for i := range hits {
			names[i] = hits[i].Service
		}
		return Endpoint{}, fmt.Errorf("endpoint %s %s matches services %s; set service", method, path, strings.Join(names, ", "))
	}
}

// Schemas lists schema names. service filters to one microservice when set.
func (s *SpecStore) Schemas(service string) ([]SchemaSummary, error) {
	var out []SchemaSummary
	matchedService := service == ""
	for i := range s.services {
		svc := &s.services[i]
		if service != "" && !strings.EqualFold(svc.Name, service) {
			continue
		}
		matchedService = true
		for _, schema := range svc.Schemas {
			out = append(out, SchemaSummary{
				Service:     svc.Name,
				Name:        schema.Name,
				Description: schema.Description,
				Type:        schema.Type,
			})
		}
	}
	if !matchedService {
		return nil, fmt.Errorf("unknown service %q", service)
	}
	if out == nil {
		out = []SchemaSummary{}
	}
	return out, nil
}

// Schema returns one component schema. name may be "Product" or "service/Product".
// A bare name must match exactly one loaded document.
func (s *SpecStore) Schema(service, name string) (SchemaDefinition, error) {
	service, name = splitSchemaName(s, service, name)
	if name == "" {
		return SchemaDefinition{}, fmt.Errorf("schema name is required")
	}
	var hits []SchemaDefinition
	for i := range s.services {
		svc := &s.services[i]
		if service != "" && !strings.EqualFold(svc.Name, service) {
			continue
		}
		for _, schema := range svc.Schemas {
			if schema.Name == name {
				hits = append(hits, schema)
			}
		}
	}
	switch len(hits) {
	case 0:
		if service != "" {
			return SchemaDefinition{}, fmt.Errorf("schema %q not found on service %s", name, service)
		}
		return SchemaDefinition{}, fmt.Errorf("schema %q not found", name)
	case 1:
		return hits[0], nil
	default:
		names := make([]string, len(hits))
		for i := range hits {
			names[i] = hits[i].Service
		}
		return SchemaDefinition{}, fmt.Errorf("schema %q matches services %s; use service/name", name, strings.Join(names, ", "))
	}
}

// Auth returns security schemes and global requirements for every service.
func (s *SpecStore) Auth() AuthInfo {
	out := AuthInfo{Services: make([]ServiceAuth, 0, len(s.services))}
	for i := range s.services {
		svc := &s.services[i]
		item := ServiceAuth{
			Service:        svc.Name,
			Schemes:        svc.Security,
			GlobalSecurity: svc.GlobalAuth,
		}
		if item.Schemes == nil {
			item.Schemes = []SecurityScheme{}
		}
		if len(item.Schemes) == 0 {
			item.Note = authNote(svc.Description)
		}
		out.Services = append(out.Services, item)
	}
	return out
}

// Errors lists 4xx, 5xx, and default responses. path limits the list to one
// path. service limits it to one microservice. An unknown path or service is an error.
func (s *SpecStore) Errors(service, path string) ([]ErrorResponse, error) {
	path = strings.TrimSpace(path)
	if service != "" && !s.knownService(service) {
		return nil, fmt.Errorf("unknown service %q", service)
	}
	var out []ErrorResponse
	seenPath := path == ""
	for i := range s.services {
		svc := &s.services[i]
		if service != "" && !strings.EqualFold(svc.Name, service) {
			continue
		}
		for _, ep := range svc.Endpoints {
			if path != "" && ep.Path != path {
				continue
			}
			if path != "" {
				seenPath = true
			}
			for _, resp := range ep.Responses {
				if !isErrorStatus(resp.Status) {
					continue
				}
				out = append(out, ErrorResponse{
					Service:     ep.Service,
					Method:      ep.Method,
					Path:        ep.Path,
					Status:      resp.Status,
					Description: resp.Description,
					ContentType: resp.ContentType,
					SchemaRef:   resp.Schema.Ref,
					SchemaType:  resp.Schema.Type,
				})
			}
		}
	}
	if path != "" && !seenPath {
		if service != "" {
			return nil, fmt.Errorf("path %s not found on service %s", path, service)
		}
		return nil, fmt.Errorf("path %s not found", path)
	}
	if out == nil {
		out = []ErrorResponse{}
	}
	return out, nil
}

func (svc *Service) summary() ServiceSummary {
	tags := svc.Tags
	if tags == nil {
		tags = []Tag{}
	}
	return ServiceSummary{
		Name:          svc.Name,
		Title:         svc.Title,
		Version:       svc.Version,
		Description:   svc.Description,
		BaseURL:       svc.BaseURL,
		Tags:          tags,
		EndpointCount: len(svc.Endpoints),
		SchemaCount:   len(svc.Schemas),
	}
}

func (s *SpecStore) knownService(name string) bool {
	_, ok := s.byName[strings.ToLower(name)]
	return ok
}

func matchesService(svc *Service, ep Endpoint, service string) bool {
	if strings.EqualFold(svc.Name, service) || strings.EqualFold(svc.Title, service) {
		return true
	}
	for _, tag := range ep.Tags {
		if strings.EqualFold(tag, service) {
			return true
		}
	}
	return false
}

func matchesKeyword(ep Endpoint, keyword string) bool {
	haystack := []string{
		ep.Path,
		ep.Method,
		ep.Summary,
		ep.Description,
		ep.OperationID,
		ep.Service,
		strings.Join(ep.Tags, " "),
	}
	for _, part := range haystack {
		if strings.Contains(strings.ToLower(part), keyword) {
			return true
		}
	}
	return false
}

func splitSchemaName(s *SpecStore, service, name string) (string, string) {
	name = strings.TrimSpace(name)
	service = strings.TrimSpace(service)
	if service != "" || name == "" {
		return service, name
	}
	for _, sep := range []string{"/", "."} {
		left, right, ok := strings.Cut(name, sep)
		if !ok || left == "" || right == "" {
			continue
		}
		if s.knownService(left) {
			return left, right
		}
	}
	return "", name
}

func isErrorStatus(status string) bool {
	if status == "default" {
		return true
	}
	code, err := strconv.Atoi(status)
	return err == nil && code >= 400
}

func authNote(description string) string {
	const gateway = "No OpenAPI securitySchemes are declared. Requests are expected to arrive through yc-gateway, which sets X-YC-Tenant-ID, X-YC-Actor-ID, X-YC-Actor-Type, and X-YC-Correlation-ID."
	if strings.Contains(description, "yc-gateway") || strings.Contains(description, "X-YC-") {
		return gateway
	}
	return "No OpenAPI security schemes are declared."
}
