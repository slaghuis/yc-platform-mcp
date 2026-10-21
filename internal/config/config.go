// Package config loads the MCP server settings from the environment.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultSpecPath  = "./specs"
	defaultTransport = "stdio"
	defaultHTTPAddr  = "127.0.0.1:8080"
)

// Config is the process configuration.
type Config struct {
	// SpecPath is a file or a directory of OpenAPI documents.
	SpecPath string
	// Transport is "stdio" or "http". HTTP uses the streamable HTTP transport.
	Transport string
	// HTTPAddr is the listen address used when Transport is "http".
	HTTPAddr string
}

// Load reads .env when that file exists, then reads the environment.
// OPENAPI_SPEC_PATH defaults to ./specs. MCP_TRANSPORT defaults to stdio.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}
	cfg := Config{
		SpecPath:  strings.TrimSpace(os.Getenv("OPENAPI_SPEC_PATH")),
		Transport: strings.ToLower(strings.TrimSpace(os.Getenv("MCP_TRANSPORT"))),
		HTTPAddr:  strings.TrimSpace(os.Getenv("MCP_HTTP_ADDR")),
	}
	if cfg.SpecPath == "" {
		cfg.SpecPath = defaultSpecPath
	}
	if cfg.Transport == "" {
		cfg.Transport = defaultTransport
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	switch cfg.Transport {
	case "stdio", "http":
	default:
		return Config{}, fmt.Errorf("unsupported MCP_TRANSPORT %q (want stdio or http)", cfg.Transport)
	}
	return cfg, nil
}
