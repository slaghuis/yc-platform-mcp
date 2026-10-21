package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("OPENAPI_SPEC_PATH", "")
	t.Setenv("MCP_TRANSPORT", "")
	t.Setenv("MCP_HTTP_ADDR", "")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "./specs", cfg.SpecPath)
	require.Equal(t, "stdio", cfg.Transport)
	require.Equal(t, "127.0.0.1:8080", cfg.HTTPAddr)
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("OPENAPI_SPEC_PATH", "/tmp/specs")
	t.Setenv("MCP_TRANSPORT", "HTTP")
	t.Setenv("MCP_HTTP_ADDR", ":9090")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "/tmp/specs", cfg.SpecPath)
	require.Equal(t, "http", cfg.Transport)
	require.Equal(t, ":9090", cfg.HTTPAddr)
}

func TestLoadRejectsUnknownTransport(t *testing.T) {
	t.Setenv("MCP_TRANSPORT", "sse")
	_, err := Load()
	require.ErrorContains(t, err, "unsupported MCP_TRANSPORT")
}
