// Package resources registers MCP resources that expose the OpenAPI model.
package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/slaghuis/yc-platform-mcp/internal/spec"
)

const (
	// OverviewURI is the high-level API summary resource.
	OverviewURI = "api://overview"
	// EndpointsURI is the complete endpoint list resource.
	EndpointsURI = "api://endpoints"
	// SchemasURI lists schema names and short descriptions.
	SchemasURI = "api://schemas"
)

// Register adds the API documentation resources to server.
func Register(server *mcp.Server, store *spec.SpecStore) {
	add(server, OverviewURI, "api-overview", "High-level API summary: title, version, description, base URL, and endpoint count.", func() any {
		return store.Overview()
	})
	add(server, EndpointsURI, "api-endpoints", "Every operation across the loaded microservice OpenAPI documents.", func() any {
		return map[string]any{"endpoints": store.Search("", "")}
	})
	add(server, SchemasURI, "api-schemas", "Schema names and short descriptions for every loaded microservice.", func() any {
		schemas, err := store.Schemas("")
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		return map[string]any{"schemas": schemas}
	})
}

func add(server *mcp.Server, uri, name, description string, payload func() any) {
	server.AddResource(&mcp.Resource{
		URI:         uri,
		Name:        name,
		Title:       name,
		Description: description,
		MIMEType:    "application/json",
	}, func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		if req != nil && req.Params != nil && req.Params.URI != "" && req.Params.URI != uri {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}
		body, err := json.MarshalIndent(payload(), "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal resource %s: %w", uri, err)
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: "application/json",
				Text:     string(body),
			}},
		}, nil
	})
}
