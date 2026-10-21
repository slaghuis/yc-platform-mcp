package spec

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func samplePath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "sample-openapi.json")
}

func TestLoadSample(t *testing.T) {
	store, err := Load(samplePath(t))
	require.NoError(t, err)

	services := store.Services()
	require.Len(t, services, 1)
	require.Equal(t, "sample-openapi", services[0].Name)
	require.Equal(t, "Sample API", services[0].Title)
	require.Equal(t, "1.2.3", services[0].Version)
	require.Equal(t, "https://api.example.com/sample", services[0].BaseURL)
	require.Equal(t, 3, services[0].EndpointCount)
	require.Equal(t, "products", services[0].Tags[0].Name)

	overview := store.Overview()
	require.Equal(t, "Sample API", overview.Title)
	require.Equal(t, "1.2.3", overview.Version)
	require.Equal(t, 3, overview.EndpointCount)
	require.Equal(t, "https://api.example.com/sample", overview.BaseURL)

	product, err := store.Schema("", "Product")
	require.NoError(t, err)
	require.Equal(t, "object", product.Type)
	require.Equal(t, "A sellable product", product.Description)
	var status SchemaDefinition
	for _, prop := range product.Properties {
		if prop.Name == "status" {
			status = prop.Schema
		}
	}
	require.Equal(t, "string", status.Type)
	require.Equal(t, []string{"active", "inactive"}, status.Enum)
	require.Equal(t, "#/components/schemas/Status", status.Ref)

	node, err := store.Schema("", "Node")
	require.NoError(t, err)
	var child SchemaDefinition
	for _, prop := range node.Properties {
		if prop.Name == "child" {
			child = prop.Schema
		}
	}
	require.Equal(t, "#/components/schemas/Node", child.Ref)
	require.Empty(t, child.Properties)

	ep, err := store.Endpoint("", "/products/{id}", "get")
	require.NoError(t, err)
	require.Equal(t, "getProduct", ep.OperationID)
	require.Len(t, ep.Parameters, 1)
	require.Equal(t, "id", ep.Parameters[0].Name)
	require.Equal(t, "path", ep.Parameters[0].In)
	require.True(t, ep.Parameters[0].Required)
	require.Equal(t, "uuid", ep.Parameters[0].Schema.Format)
	require.Equal(t, "global", ep.SecuritySource)
	require.Equal(t, "bearerAuth", ep.Security[0].Schemes[0].Name)

	missing, err := store.Schema("", "Nope")
	require.Error(t, err)
	require.Empty(t, missing.Name)

	_, err = store.Endpoint("", "/missing", "GET")
	require.ErrorContains(t, err, "not found")

	_, err = store.Endpoint("", "/products", "FOO")
	require.ErrorContains(t, err, "not found")
}

func TestLoadFailures(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	require.ErrorContains(t, err, "stat spec path")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi"), 0o644))
	_, err = Load(dir)
	require.ErrorContains(t, err, "no OpenAPI files")

	badJSON := filepath.Join(t.TempDir(), "broken.json")
	require.NoError(t, os.WriteFile(badJSON, []byte("{"), 0o644))
	_, err = Load(badJSON)
	require.ErrorContains(t, err, "invalid JSON")

	notOpenAPI := filepath.Join(t.TempDir(), "plain.json")
	require.NoError(t, os.WriteFile(notOpenAPI, []byte(`{"hello":"world"}`), 0o644))
	_, err = Load(notOpenAPI)
	require.ErrorContains(t, err, "parse OpenAPI")

	badYAML := filepath.Join(t.TempDir(), "broken.yaml")
	require.NoError(t, os.WriteFile(badYAML, []byte(":\n  - ["), 0o644))
	_, err = Load(badYAML)
	require.ErrorContains(t, err, "invalid YAML")

	txt := filepath.Join(t.TempDir(), "spec.txt")
	require.NoError(t, os.WriteFile(txt, []byte("nope"), 0o644))
	_, err = Load(txt)
	require.ErrorContains(t, err, "unsupported spec extension")
}

func TestLoadDirectoryYAMLAndDuplicateService(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "yc-yaml.yaml"), []byte(`
openapi: 3.0.3
info:
  title: yc-yaml
  version: 0.0.1
  description: yaml fixture
paths:
  /ping:
    get:
      summary: Ping
      responses:
        "200":
          description: ok
`), 0o644))
	store, err := Load(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"yc-yaml"}, names(store))
	found := store.Search("ping", "yc-yaml")
	require.Len(t, found, 1)
	require.Equal(t, "GET", found[0].Method)

	dup := filepath.Join(dir, "nested")
	require.NoError(t, os.MkdirAll(dup, 0o755))
	body := []byte("openapi: 3.0.3\ninfo:\n  title: same\n  version: '1'\npaths: {}\n")
	require.NoError(t, os.WriteFile(filepath.Join(dup, "openapi.yaml"), body, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dup, "nested.json"), []byte(`{"openapi":"3.0.3","info":{"title":"same","version":"1"},"paths":{}}`), 0o644))
	_, err = Load(dup)
	require.ErrorContains(t, err, "duplicate service")
}

func TestLoadPlatformSpecs(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "specs")
	store, err := Load(dir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(store.Services()), 5)
	overview := store.Overview()
	require.Equal(t, "yc-platform", overview.Title)
	require.Greater(t, overview.EndpointCount, 0)
	product, err := store.Schema("yc-product", "Product")
	require.NoError(t, err)
	require.NotEmpty(t, product.Properties)
}

func TestSearchSchemaAndErrors(t *testing.T) {
	store, err := Load(samplePath(t))
	require.NoError(t, err)

	found := store.Search("list products", "products")
	require.Len(t, found, 1)
	require.Equal(t, "/products", found[0].Path)
	require.Empty(t, store.Search("list products", "missing-service"))

	schemas, err := store.Schemas("")
	require.NoError(t, err)
	require.NotEmpty(t, schemas)
	_, err = store.Schemas("nope")
	require.ErrorContains(t, err, "unknown service")

	qualified, err := store.Schema("", "sample-openapi/Product")
	require.NoError(t, err)
	require.Equal(t, "Product", qualified.Name)

	errors, err := store.Errors("", "/products")
	require.NoError(t, err)
	require.NotEmpty(t, errors)
	require.Equal(t, "400", errors[0].Status)
	_, err = store.Errors("", "/nope")
	require.ErrorContains(t, err, "not found")

	auth := store.Auth()
	require.Equal(t, "bearerAuth", auth.Services[0].Schemes[0].Name)
	require.Equal(t, "http", auth.Services[0].Schemes[0].Type)
	require.Equal(t, "JWT", auth.Services[0].Schemes[0].BearerFormat)
}

func TestServerVariablesAndGatewayAuthNote(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "yc-gateway.yaml"), []byte(`
openapi: 3.1.0
info:
  title: yc-gateway
  version: "1"
  description: Routes are reached through yc-gateway, which sets X-YC-Tenant-ID.
servers:
  - url: https://{host}/v1
    variables:
      host:
        default: shop.example
paths:
  /health:
    get:
      responses:
        "204":
          description: ok
`), 0o644))
	store, err := Load(dir)
	require.NoError(t, err)
	require.Equal(t, "https://shop.example/v1", store.Services()[0].BaseURL)
	auth := store.Auth()
	require.Contains(t, auth.Services[0].Note, "yc-gateway")
	errors, err := store.Errors("", "")
	require.NoError(t, err)
	require.Empty(t, errors)
}

func names(store *SpecStore) []string {
	services := store.Services()
	out := make([]string, len(services))
	for i := range services {
		out[i] = services[i].Name
	}
	return out
}
