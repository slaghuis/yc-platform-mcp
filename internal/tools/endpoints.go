package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/slaghuis/yc-platform-mcp/internal/spec"
)

// GetEndpointDetailsInput identifies one operation.
type GetEndpointDetailsInput struct {
	Path    string `json:"path" jsonschema:"OpenAPI path, such as /products/{id}"`
	Method  string `json:"method" jsonschema:"HTTP method, such as GET or POST"`
	Service string `json:"service,omitempty" jsonschema:"microservice name, required when the path exists on more than one service"`
}

// GetEndpointDetails returns the full operation contract.
func (h *Handler) GetEndpointDetails(ctx context.Context, _ *mcp.CallToolRequest, in GetEndpointDetailsInput) (*mcp.CallToolResult, any, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	method, err := normalizeMethod(in.Method)
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.Path) == "" {
		return nil, nil, fmt.Errorf("path is required")
	}
	endpoint, err := h.Store.Endpoint(in.Service, in.Path, method)
	if err != nil {
		return nil, nil, err
	}
	return nil, endpoint, nil
}

// ListErrorResponsesInput selects error responses.
type ListErrorResponsesInput struct {
	Path    string `json:"path,omitempty" jsonschema:"optional OpenAPI path; omit to list errors for every operation"`
	Service string `json:"service,omitempty" jsonschema:"optional microservice name"`
}

// ListErrorResponsesOutput is the list_error_responses result.
type ListErrorResponsesOutput struct {
	Count  int                  `json:"count"`
	Errors []spec.ErrorResponse `json:"errors"`
}

// ListErrorResponses lists 4xx, 5xx, and default responses.
func (h *Handler) ListErrorResponses(ctx context.Context, _ *mcp.CallToolRequest, in ListErrorResponsesInput) (*mcp.CallToolResult, ListErrorResponsesOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, ListErrorResponsesOutput{}, err
	}
	errors, err := h.Store.Errors(in.Service, in.Path)
	if err != nil {
		return nil, ListErrorResponsesOutput{}, err
	}
	return nil, ListErrorResponsesOutput{Count: len(errors), Errors: errors}, nil
}
