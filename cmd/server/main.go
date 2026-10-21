// Command server is the yc-platform OpenAPI documentation MCP server.
// It speaks MCP on stdio by default. Set MCP_TRANSPORT=http to serve
// streamable HTTP instead.
package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/slaghuis/yc-platform-mcp/internal/config"
	"github.com/slaghuis/yc-platform-mcp/internal/server"
	"github.com/slaghuis/yc-platform-mcp/internal/spec"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration", "err", err)
		os.Exit(1)
	}
	store, err := spec.Load(cfg.SpecPath)
	if err != nil {
		slog.Error("load openapi specs", "path", cfg.SpecPath, "err", err)
		os.Exit(1)
	}
	slog.Info("loaded openapi specs", "path", cfg.SpecPath, "services", len(store.Services()))

	srv := server.New(store)
	if err := server.Run(context.Background(), cfg, srv); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		slog.Error("mcp server stopped", "err", err)
		os.Exit(1)
	}
}
