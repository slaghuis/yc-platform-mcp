package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/slaghuis/yc-platform-mcp/internal/spec"
)

// ListSchemasInput filters schema names.
type ListSchemasInput struct {
	Service string `json:"service,omitempty" jsonschema:"optional microservice name"`
}

// ListSchemasOutput is the list_schemas result.
type ListSchemasOutput struct {
	Count   int                  `json:"count"`
	Schemas []spec.SchemaSummary `json:"schemas"`
}

// ListSchemas lists component schema names and descriptions.
func (h *Handler) ListSchemas(ctx context.Context, _ *mcp.CallToolRequest, in ListSchemasInput) (*mcp.CallToolResult, ListSchemasOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, ListSchemasOutput{}, err
	}
	schemas, err := h.Store.Schemas(in.Service)
	if err != nil {
		return nil, ListSchemasOutput{}, err
	}
	return nil, ListSchemasOutput{Count: len(schemas), Schemas: schemas}, nil
}

// GetSchemaInput names one component schema.
type GetSchemaInput struct {
	Name    string `json:"name" jsonschema:"schema name, or service/name when the same name exists on more than one microservice"`
	Service string `json:"service,omitempty" jsonschema:"optional microservice name"`
}

// GetSchema returns a resolved component schema.
func (h *Handler) GetSchema(ctx context.Context, _ *mcp.CallToolRequest, in GetSchemaInput) (*mcp.CallToolResult, any, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, nil, fmt.Errorf("schema name is required")
	}
	schema, err := h.Store.Schema(in.Service, in.Name)
	if err != nil {
		return nil, nil, err
	}
	return nil, schema, nil
}
