// Package spec loads OpenAPI 3.x documents and keeps a resolved model for MCP tools.
package spec

// SpecStore is the parsed OpenAPI model for every loaded microservice.
// Build it once with Load and share it with tool handlers. It is safe for
// concurrent reads after Load returns.
type SpecStore struct {
	services []Service
	byName   map[string]int
}

// Service is one OpenAPI document, typically one yc-platform microservice.
type Service struct {
	Name        string                `json:"name"`
	Title       string                `json:"title"`
	Version     string                `json:"version"`
	Description string                `json:"description,omitempty"`
	OpenAPI     string                `json:"openapi,omitempty"`
	BaseURL     string                `json:"base_url,omitempty"`
	Servers     []string              `json:"servers,omitempty"`
	Tags        []Tag                 `json:"tags"`
	Endpoints   []Endpoint            `json:"endpoints,omitempty"`
	Schemas     []SchemaDefinition    `json:"schemas,omitempty"`
	Security    []SecurityScheme      `json:"security_schemes,omitempty"`
	GlobalAuth  []SecurityRequirement `json:"global_security,omitempty"`
}

// Tag is an OpenAPI tag, or a tag used by an operation and not declared.
type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Endpoint is one HTTP operation with parameters, body, responses, and auth.
type Endpoint struct {
	Service        string                `json:"service"`
	Method         string                `json:"method"`
	Path           string                `json:"path"`
	OperationID    string                `json:"operation_id,omitempty"`
	Summary        string                `json:"summary,omitempty"`
	Description    string                `json:"description,omitempty"`
	Tags           []string              `json:"tags,omitempty"`
	Deprecated     bool                  `json:"deprecated,omitempty"`
	Parameters     []Parameter           `json:"parameters,omitempty"`
	RequestBody    *RequestBody          `json:"request_body,omitempty"`
	Responses      []Response            `json:"responses,omitempty"`
	Security       []SecurityRequirement `json:"security,omitempty"`
	SecuritySource string                `json:"security_source,omitempty"`
}

// EndpointSummary is an operation without its body and response schemas.
type EndpointSummary struct {
	Service     string   `json:"service"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OperationID string   `json:"operation_id,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
}

// Parameter is a path, query, header, or cookie parameter.
type Parameter struct {
	Name        string           `json:"name"`
	In          string           `json:"in"`
	Description string           `json:"description,omitempty"`
	Required    bool             `json:"required"`
	Deprecated  bool             `json:"deprecated,omitempty"`
	Schema      SchemaDefinition `json:"schema"`
}

// RequestBody is the preferred media type of an operation request body.
type RequestBody struct {
	Required    bool             `json:"required"`
	Description string           `json:"description,omitempty"`
	ContentType string           `json:"content_type,omitempty"`
	Schema      SchemaDefinition `json:"schema"`
}

// Response is one operation response, including error statuses.
type Response struct {
	Status      string           `json:"status"`
	Description string           `json:"description,omitempty"`
	ContentType string           `json:"content_type,omitempty"`
	Schema      SchemaDefinition `json:"schema,omitempty"`
}

// SchemaDefinition is a JSON Schema object with local $ref values resolved.
// Circular references stop at Ref instead of expanding forever.
type SchemaDefinition struct {
	Service                     string             `json:"service,omitempty"`
	Name                        string             `json:"name,omitempty"`
	Description                 string             `json:"description,omitempty"`
	Type                        string             `json:"type,omitempty"`
	Format                      string             `json:"format,omitempty"`
	Required                    []string           `json:"required,omitempty"`
	Enum                        []string           `json:"enum,omitempty"`
	Ref                         string             `json:"ref,omitempty"`
	Properties                  []SchemaProperty   `json:"properties,omitempty"`
	Items                       *SchemaDefinition  `json:"items,omitempty"`
	AdditionalProperties        *SchemaDefinition  `json:"additional_properties,omitempty"`
	AdditionalPropertiesAllowed *bool              `json:"additional_properties_allowed,omitempty"`
	AllOf                       []SchemaDefinition `json:"all_of,omitempty"`
	OneOf                       []SchemaDefinition `json:"one_of,omitempty"`
	AnyOf                       []SchemaDefinition `json:"any_of,omitempty"`
}

// SchemaProperty is one field of an object schema.
type SchemaProperty struct {
	Name   string           `json:"name"`
	Schema SchemaDefinition `json:"schema"`
}

// SchemaSummary is a schema name and a short description.
type SchemaSummary struct {
	Service     string `json:"service"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
}

// SecurityScheme is one entry from components.securitySchemes.
type SecurityScheme struct {
	Name             string `json:"name"`
	Type             string `json:"type"`
	Description      string `json:"description,omitempty"`
	In               string `json:"in,omitempty"`
	ParameterName    string `json:"parameter_name,omitempty"`
	Scheme           string `json:"scheme,omitempty"`
	BearerFormat     string `json:"bearer_format,omitempty"`
	OpenIDConnectURL string `json:"open_id_connect_url,omitempty"`
}

// SecurityRequirement is one alternative (OR) security requirement.
// Schemes inside it must all be satisfied (AND).
type SecurityRequirement struct {
	Schemes []SecuritySchemeUse `json:"schemes"`
}

// SecuritySchemeUse names a scheme and the OAuth scopes it requires.
type SecuritySchemeUse struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes,omitempty"`
}

// ServiceSummary is the list_services view of a microservice.
type ServiceSummary struct {
	Name          string `json:"name"`
	Title         string `json:"title"`
	Version       string `json:"version"`
	Description   string `json:"description,omitempty"`
	BaseURL       string `json:"base_url,omitempty"`
	Tags          []Tag  `json:"tags"`
	EndpointCount int    `json:"endpoint_count"`
	SchemaCount   int    `json:"schema_count"`
}

// Overview is the high-level description returned by get_api_overview.
type Overview struct {
	Title         string           `json:"title"`
	Version       string           `json:"version"`
	Description   string           `json:"description"`
	BaseURL       string           `json:"base_url"`
	EndpointCount int              `json:"endpoint_count"`
	SchemaCount   int              `json:"schema_count"`
	Services      []ServiceSummary `json:"services"`
}

// ServiceAuth is the authentication picture for one microservice.
type ServiceAuth struct {
	Service        string                `json:"service"`
	Schemes        []SecurityScheme      `json:"schemes"`
	GlobalSecurity []SecurityRequirement `json:"global_security,omitempty"`
	Note           string                `json:"note,omitempty"`
}

// AuthInfo is the authentication picture for every loaded microservice.
type AuthInfo struct {
	Services []ServiceAuth `json:"services"`
}

// ErrorResponse is one 4xx, 5xx, or default response.
type ErrorResponse struct {
	Service     string `json:"service"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	SchemaRef   string `json:"schema_ref,omitempty"`
	SchemaType  string `json:"schema_type,omitempty"`
}

// Brief returns the endpoint without its nested schemas.
func (e Endpoint) Brief() EndpointSummary {
	return EndpointSummary{
		Service:     e.Service,
		Method:      e.Method,
		Path:        e.Path,
		OperationID: e.OperationID,
		Summary:     e.Summary,
		Tags:        e.Tags,
		Deprecated:  e.Deprecated,
	}
}
