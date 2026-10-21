package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/slaghuis/yc-platform-mcp/internal/spec"
	"github.com/stretchr/testify/require"
)

func newHandler(t *testing.T) *Handler {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "sample-openapi.json")
	store, err := spec.Load(path)
	require.NoError(t, err)
	return NewHandler(store)
}

func TestToolHandlers(t *testing.T) {
	h := newHandler(t)
	ctx := context.Background()

	_, services, err := h.ListServices(ctx, nil, ListServicesInput{})
	require.NoError(t, err)
	require.Equal(t, "sample-openapi", services.Services[0].Name)
	require.Equal(t, 3, services.Services[0].EndpointCount)

	_, overview, err := h.GetAPIOverview(ctx, nil, GetAPIOverviewInput{})
	require.NoError(t, err)
	require.Equal(t, "Sample API", overview.Title)
	require.Equal(t, "1.2.3", overview.Version)
	require.Equal(t, 3, overview.EndpointCount)
	require.Equal(t, "https://api.example.com/sample", overview.BaseURL)

	_, found, err := h.SearchEndpoints(ctx, nil, SearchEndpointsInput{Keyword: "product", Service: "products"})
	require.NoError(t, err)
	require.NotEmpty(t, found.Endpoints)
	_, _, err = h.SearchEndpoints(ctx, nil, SearchEndpointsInput{})
	require.ErrorContains(t, err, "keyword is required")
	_, none, err := h.SearchEndpoints(ctx, nil, SearchEndpointsInput{Keyword: "does-not-exist"})
	require.NoError(t, err)
	require.Empty(t, none.Endpoints)

	_, details, err := h.GetEndpointDetails(ctx, nil, GetEndpointDetailsInput{Path: "/products/{id}", Method: "get"})
	require.NoError(t, err)
	endpoint := details.(spec.Endpoint)
	require.Equal(t, "getProduct", endpoint.OperationID)
	require.Equal(t, "uuid", endpoint.Parameters[0].Schema.Format)
	_, _, err = h.GetEndpointDetails(ctx, nil, GetEndpointDetailsInput{Path: "/missing", Method: "GET"})
	require.ErrorContains(t, err, "not found")
	_, _, err = h.GetEndpointDetails(ctx, nil, GetEndpointDetailsInput{Path: "/products", Method: "nope"})
	require.ErrorContains(t, err, "unsupported method")
	_, _, err = h.GetEndpointDetails(ctx, nil, GetEndpointDetailsInput{Method: "GET"})
	require.ErrorContains(t, err, "path is required")

	_, schemas, err := h.ListSchemas(ctx, nil, ListSchemasInput{})
	require.NoError(t, err)
	require.NotEmpty(t, schemas.Schemas)
	_, _, err = h.ListSchemas(ctx, nil, ListSchemasInput{Service: "missing"})
	require.ErrorContains(t, err, "unknown service")

	_, schema, err := h.GetSchema(ctx, nil, GetSchemaInput{Name: "Product"})
	require.NoError(t, err)
	require.Equal(t, "Product", schema.(spec.SchemaDefinition).Name)
	_, _, err = h.GetSchema(ctx, nil, GetSchemaInput{Name: "Nope"})
	require.ErrorContains(t, err, "not found")
	_, _, err = h.GetSchema(ctx, nil, GetSchemaInput{})
	require.ErrorContains(t, err, "schema name is required")

	_, auth, err := h.GetAuthInfo(ctx, nil, GetAuthInfoInput{})
	require.NoError(t, err)
	require.Equal(t, "bearerAuth", auth.Services[0].Schemes[0].Name)

	_, errors, err := h.ListErrorResponses(ctx, nil, ListErrorResponsesInput{Path: "/products"})
	require.NoError(t, err)
	require.NotEmpty(t, errors.Errors)
	_, all, err := h.ListErrorResponses(ctx, nil, ListErrorResponsesInput{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, all.Count, errors.Count)
	_, _, err = h.ListErrorResponses(ctx, nil, ListErrorResponsesInput{Path: "/missing"})
	require.ErrorContains(t, err, "not found")

	_, curl, err := h.GetExampleRequest(ctx, nil, GetExampleRequestInput{Path: "/products", Method: "POST"})
	require.NoError(t, err)
	require.Equal(t, "curl", curl.Format)
	require.Contains(t, curl.Example, "curl -X POST")
	require.Contains(t, curl.Example, "Authorization: Bearer <token>")
	require.Contains(t, curl.Example, `"name"`)
	require.False(t, curl.BaseURLAssumed)

	_, goExample, err := h.GetExampleRequest(ctx, nil, GetExampleRequestInput{Path: "/products/{id}", Method: "GET", Format: "go"})
	require.NoError(t, err)
	require.Contains(t, goExample.Example, "http.NewRequest")
	require.Contains(t, goExample.URL, "00000000-0000-0000-0000-000000000000")

	_, js, err := h.GetExampleRequest(ctx, nil, GetExampleRequestInput{Path: "/products", Method: "get", Format: "javascript"})
	require.NoError(t, err)
	require.Contains(t, js.Example, "fetch(")
	_, _, err = h.GetExampleRequest(ctx, nil, GetExampleRequestInput{Path: "/products", Method: "GET", Format: "python"})
	require.ErrorContains(t, err, "unsupported format")
	_, _, err = h.GetExampleRequest(ctx, nil, GetExampleRequestInput{Path: "/missing", Method: "GET", Format: "curl"})
	require.ErrorContains(t, err, "not found")
}

func TestAmbiguousEndpointAndSchema(t *testing.T) {
	dir := t.TempDir()
	body := `{
	  "openapi": "3.0.3",
	  "info": {"title": "%s", "version": "1.0.0"},
	  "paths": {
	    "/widgets": {
	      "get": {
	        "summary": "List widgets",
	        "responses": {"200": {"description": "ok"}, "500": {"description": "failed"}}
	      }
	    }
	  },
	  "components": {"schemas": {"Widget": {"type": "object", "description": "a widget"}}}
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "yc-a.json"), []byte(fmt.Sprintf(body, "yc-a")), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "yc-b.json"), []byte(fmt.Sprintf(body, "yc-b")), 0o644))
	store, err := spec.Load(dir)
	require.NoError(t, err)
	h := NewHandler(store)
	ctx := context.Background()

	_, _, err = h.GetEndpointDetails(ctx, nil, GetEndpointDetailsInput{Path: "/widgets", Method: "GET"})
	require.ErrorContains(t, err, "matches services")
	_, got, err := h.GetEndpointDetails(ctx, nil, GetEndpointDetailsInput{Path: "/widgets", Method: "GET", Service: "yc-b"})
	require.NoError(t, err)
	require.Equal(t, "yc-b", got.(spec.Endpoint).Service)

	_, _, err = h.GetSchema(ctx, nil, GetSchemaInput{Name: "Widget"})
	require.ErrorContains(t, err, "matches services")
	_, schema, err := h.GetSchema(ctx, nil, GetSchemaInput{Name: "yc-a/Widget"})
	require.NoError(t, err)
	require.Equal(t, "yc-a", schema.(spec.SchemaDefinition).Service)

	_, example, err := h.GetExampleRequest(ctx, nil, GetExampleRequestInput{Path: "/widgets", Method: "GET", Service: "yc-a", Format: "js"})
	require.NoError(t, err)
	require.True(t, example.BaseURLAssumed)
	require.Contains(t, example.URL, "https://api.example.com/widgets")
}

func TestCancelledContext(t *testing.T) {
	h := newHandler(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := h.ListServices(ctx, nil, ListServicesInput{})
	require.ErrorIs(t, err, context.Canceled)
	_, _, err = h.SearchEndpoints(ctx, nil, SearchEndpointsInput{Keyword: "x"})
	require.ErrorIs(t, err, context.Canceled)
	_, _, err = h.GetEndpointDetails(ctx, nil, GetEndpointDetailsInput{Path: "/products", Method: "GET"})
	require.ErrorIs(t, err, context.Canceled)
	_, _, err = h.GetSchema(ctx, nil, GetSchemaInput{Name: "Product"})
	require.ErrorIs(t, err, context.Canceled)
	_, _, err = h.GetExampleRequest(ctx, nil, GetExampleRequestInput{Path: "/products", Method: "GET"})
	require.ErrorIs(t, err, context.Canceled)
}
