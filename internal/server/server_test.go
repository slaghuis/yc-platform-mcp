package server_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/slaghuis/yc-platform-mcp/internal/config"
	"github.com/slaghuis/yc-platform-mcp/internal/server"
	"github.com/slaghuis/yc-platform-mcp/internal/spec"
	"github.com/stretchr/testify/require"
)

func TestSessionListsToolsAndReadsResources(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	store, err := spec.Load(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "sample-openapi.json"))
	require.NoError(t, err)

	srv := server.New(store)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	_, err = srv.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_api_overview",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	var overview spec.Overview
	require.NoError(t, json.Unmarshal(structured(t, result), &overview))
	require.Equal(t, "Sample API", overview.Title)
	require.Equal(t, 3, overview.EndpointCount)

	for _, uri := range []string{"api://overview", "api://endpoints", "api://schemas"} {
		resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		require.NoError(t, err)
		require.NotEmpty(t, resource.Contents)
		require.NotEmpty(t, resource.Contents[0].Text)
		require.True(t, json.Valid([]byte(resource.Contents[0].Text)))
	}
}

func TestStdioBinary(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(file), "..", "..")
	bin := filepath.Join(t.TempDir(), "yc-platform-mcp")
	build := exec.Command("go", "build", "-o", bin, "./cmd/server")
	build.Dir = root
	out, err := build.CombinedOutput()
	require.NoError(t, err, string(out))

	run := exec.Command(bin)
	run.Env = append(os.Environ(), "OPENAPI_SPEC_PATH="+filepath.Join(root, "testdata", "sample-openapi.json"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "stdio-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: run}, nil)
	require.NoError(t, err)
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_services",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotEmpty(t, result.Content)
}

func TestHTTPTransportStops(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	store, err := spec.Load(filepath.Join(filepath.Dir(file), "..", "..", "testdata", "sample-openapi.json"))
	require.NoError(t, err)
	srv := server.New(store)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx, config.Config{Transport: "http", HTTPAddr: "127.0.0.1:0"}, srv)
	}()
	time.Sleep(150 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("http transport did not stop")
	}
}

func TestRunRejectsUnknownTransport(t *testing.T) {
	err := server.Run(context.Background(), config.Config{Transport: "sse"}, nil)
	require.ErrorContains(t, err, "unsupported MCP_TRANSPORT")
}

func structured(t *testing.T, result *mcp.CallToolResult) []byte {
	t.Helper()
	switch value := result.StructuredContent.(type) {
	case json.RawMessage:
		return value
	case []byte:
		return value
	default:
		body, err := json.Marshal(value)
		require.NoError(t, err)
		return body
	}
}
