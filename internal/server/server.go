// Package server wires the MCP server and runs it on a transport.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/slaghuis/yc-platform-mcp/internal/config"
	"github.com/slaghuis/yc-platform-mcp/internal/resources"
	"github.com/slaghuis/yc-platform-mcp/internal/spec"
	"github.com/slaghuis/yc-platform-mcp/internal/tools"
)

// New builds an MCP server with the documentation tools and resources registered.
// The returned server is not running. Call Run with a transport.
func New(store *spec.SpecStore) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "yc-platform-mcp",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Instructions: "Documentation for the yc-platform microservice APIs. " +
			"Use list_services and search_endpoints to find operations, " +
			"get_endpoint_details and get_schema for contracts, " +
			"and get_example_request for curl, Go, or JavaScript samples. " +
			"Resources api://overview, api://endpoints, and api://schemas expose the same model.",
		Logger: slog.Default(),
	})
	tools.Register(srv, store)
	resources.Register(srv, store)
	return srv
}

// Run serves srv until ctx is cancelled or the client disconnects.
// cfg.Transport selects stdio (the default) or streamable HTTP.
// Streamable HTTP is the transport to use when moving off stdio; the older
// HTTP/SSE transport is not implemented.
func Run(ctx context.Context, cfg config.Config, srv *mcp.Server) error {
	switch strings.ToLower(cfg.Transport) {
	case "", "stdio":
		if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
			return fmt.Errorf("stdio transport: %w", err)
		}
		return nil
	case "http":
		return runHTTP(ctx, cfg.HTTPAddr, srv)
	default:
		return fmt.Errorf("unsupported MCP_TRANSPORT %q (want stdio or http)", cfg.Transport)
	}
}

func runHTTP(ctx context.Context, addr string, srv *mcp.Server) error {
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, &mcp.StreamableHTTPOptions{
		Logger:       slog.Default(),
		JSONResponse: true,
	})
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("mcp http transport listening", "addr", addr)
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown http transport: %w", err)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("http transport: %w", err)
		}
		return nil
	}
}
