package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rahadiangg/mcp-mariadb/internal/config"
	"github.com/rahadiangg/mcp-mariadb/internal/mcp"
	"go.uber.org/zap"
)

func main() {
	// Parse CLI flags
	transport := flag.String("transport", "stdio", "Transport protocol: stdio, sse, or http")
	host := flag.String("host", "127.0.0.1", "Host for SSE or HTTP transport")
	port := flag.Int("port", 9001, "Port for SSE or HTTP transport")
	path := flag.String("path", "/mcp", "Path for HTTP transport")
	flag.Parse()

	// Load configuration
	cfg, logger, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Log startup
	logger.Info("Starting MariaDB MCP Server",
		zap.String("transport", *transport),
		zap.String("host", *host),
		zap.Int("port", *port),
		zap.String("path", *path),
		zap.Bool("read_only", cfg.ReadOnly),
	)

	// Create and run server
	server := mcp.NewServer(cfg, logger)

	// Handle shutdown gracefully
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := server.Run(ctx, *transport, *host, *port, *path); err != nil {
		logger.Error("Server error", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("Server shutdown complete")
}
