// Package tools registers the OpenAPI documentation tools exposed by the MCP server.
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/slaghuis/yc-platform-mcp/internal/spec"
)

// Handler answers MCP tool calls from a parsed OpenAPI store.
type Handler struct {
	Store *spec.SpecStore
}

// NewHandler returns a tool handler bound to store.
func NewHandler(store *spec.SpecStore) *Handler {
	return &Handler{Store: store}
}

// Register adds every documentation tool to server.
func Register(server *mcp.Server, store *spec.SpecStore) {
	h := NewHandler(store)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_services",
		Description: "List yc-platform microservices loaded from OpenAPI documents, including title, version, tags, and endpoint counts.",
	}, h.ListServices)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_endpoints",
		Description: "Search operations by keyword across path, summary, description, operationId, and tags. Optionally filter by microservice name or OpenAPI tag.",
	}, h.SearchEndpoints)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_endpoint_details",
		Description: "Return parameters, request body, responses, and security for one operation identified by path and HTTP method. Set service when the same path exists on more than one microservice.",
	}, h.GetEndpointDetails)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_schemas",
		Description: "List data-model schema names and short descriptions. Optionally filter by microservice.",
	}, h.ListSchemas)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_schema",
		Description: "Return a component schema with properties, types, formats, and enums. Local $ref values are resolved. Name may be Product or service/Product.",
	}, h.GetSchema)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_auth_info",
		Description: "Return the OpenAPI security schemes and global security requirements used by each microservice.",
	}, h.GetAuthInfo)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_example_request",
		Description: "Generate an example request for an operation. format is curl, go, or javascript and defaults to curl.",
	}, h.GetExampleRequest)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_error_responses",
		Description: "List 4xx, 5xx, and default responses for every operation, or for one path when path is set.",
	}, h.ListErrorResponses)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_api_overview",
		Description: "Return the API title, version, description, base URL, and endpoint count. Several microservice specs are summarized together and listed individually.",
	}, h.GetAPIOverview)
}

// ListServicesInput has no parameters.
type ListServicesInput struct{}

// ListServicesOutput is the list_services result.
type ListServicesOutput struct {
	Services []spec.ServiceSummary `json:"services"`
}

// ListServices lists every loaded microservice.
func (h *Handler) ListServices(ctx context.Context, _ *mcp.CallToolRequest, _ ListServicesInput) (*mcp.CallToolResult, ListServicesOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, ListServicesOutput{}, err
	}
	return nil, ListServicesOutput{Services: h.Store.Services()}, nil
}

// SearchEndpointsInput is the search_endpoints argument object.
type SearchEndpointsInput struct {
	Keyword string `json:"keyword" jsonschema:"text to match against path, method, summary, description, operationId, or tags"`
	Service string `json:"service,omitempty" jsonschema:"optional microservice name or OpenAPI tag"`
}

// SearchEndpointsOutput is the search_endpoints result.
type SearchEndpointsOutput struct {
	Count     int                    `json:"count"`
	Endpoints []spec.EndpointSummary `json:"endpoints"`
}

// SearchEndpoints finds operations by keyword and optional service or tag.
func (h *Handler) SearchEndpoints(ctx context.Context, _ *mcp.CallToolRequest, in SearchEndpointsInput) (*mcp.CallToolResult, SearchEndpointsOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, SearchEndpointsOutput{}, err
	}
	keyword := strings.TrimSpace(in.Keyword)
	if keyword == "" {
		return nil, SearchEndpointsOutput{}, fmt.Errorf("keyword is required")
	}
	endpoints := h.Store.Search(keyword, in.Service)
	return nil, SearchEndpointsOutput{Count: len(endpoints), Endpoints: endpoints}, nil
}

// GetAPIOverviewInput has no parameters.
type GetAPIOverviewInput struct{}

// GetAPIOverview returns the combined API summary.
func (h *Handler) GetAPIOverview(ctx context.Context, _ *mcp.CallToolRequest, _ GetAPIOverviewInput) (*mcp.CallToolResult, spec.Overview, error) {
	if err := ctx.Err(); err != nil {
		return nil, spec.Overview{}, err
	}
	return nil, h.Store.Overview(), nil
}

// GetAuthInfoInput has no parameters.
type GetAuthInfoInput struct{}

// GetAuthInfo returns security schemes for every loaded document.
func (h *Handler) GetAuthInfo(ctx context.Context, _ *mcp.CallToolRequest, _ GetAuthInfoInput) (*mcp.CallToolResult, spec.AuthInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, spec.AuthInfo{}, err
	}
	return nil, h.Store.Auth(), nil
}

func normalizeMethod(method string) (string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE":
		return method, nil
	default:
		return "", fmt.Errorf("unsupported method %q", method)
	}
}
