// Command edgex-foundry-mcp is a Model Context Protocol server that lets AI
// agents inspect an EdgeX Foundry (LF Edge) deployment through its REST API v3.
// It is read-only by default.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/config"
	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/tools"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr, nil))
}

// run is the testable entry point. When ready is non-nil, the HTTP listen
// address is sent on it once the server accepts connections.
func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer, ready chan<- string) int {
	cfg, err := config.Load(args, getenv, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "edgex-foundry-mcp: %v\n", err)
		return 2
	}
	if cfg.ShowVersion {
		_, _ = fmt.Fprintf(stdout, "edgex-foundry-mcp %s\n", version)
		return 0
	}

	// Logs go to stderr only: stdout carries the MCP stdio protocol.
	log := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	for _, w := range cfg.Warnings() {
		log.Warn(w)
	}

	client, err := edgex.New(edgex.Options{
		MetadataURL: cfg.MetadataURL,
		DataURL:     cfg.DataURL,
		CommandURL:  cfg.CommandURL,
		GatewayURL:  cfg.GatewayURL,
		Token:       string(cfg.Token),
		CAFile:      cfg.GatewayCAFile,
		Timeout:     cfg.Timeout,
		MaxResults:  cfg.MaxResults,
		Logger:      log,
	})
	if err != nil {
		log.Error("cannot create EdgeX client", "error", err)
		return 1
	}

	server := mcp.NewServer(
		&mcp.Implementation{Name: "edgex-foundry-mcp", Title: "EdgeX Foundry", Version: version},
		&mcp.ServerOptions{Instructions: tools.ServerInstructions(cfg.EnableWrites), Logger: log},
	)
	names, err := tools.Register(server, client, tools.Options{
		EnableWrites:       cfg.EnableWrites,
		DisableDeviceReads: cfg.DisableDeviceReads,
		Logger:             log,
	})
	if err != nil {
		log.Error("tool registration failed", "error", err)
		return 1
	}
	mode := "direct"
	if cfg.GatewayMode() {
		mode = "gateway"
	}
	log.Info("starting edgex-foundry-mcp", "version", version, "transport", cfg.Transport, "mode", mode,
		"metadata", client.BaseURL(edgex.Metadata), "data", client.BaseURL(edgex.Data), "command", client.BaseURL(edgex.Command),
		"tools", len(names))

	switch cfg.Transport {
	case config.TransportHTTP:
		err = serveHTTP(ctx, server, cfg.HTTPAddr, log, ready)
	default:
		err = server.Run(ctx, &mcp.StdioTransport{})
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("server stopped", "error", err)
		return 1
	}
	return 0
}

// serveHTTP serves the MCP streamable HTTP transport at /mcp until ctx ends.
func serveHTTP(ctx context.Context, server *mcp.Server, addr string, log *slog.Logger, ready chan<- string) error {
	// Stateless: the server keeps no per-session state. The SDK's DNS-rebinding
	// (localhost) protection stays enabled.
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, Logger: log})
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	// No WriteTimeout: streamable HTTP responses may be long-lived streams.
	hs := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 120 * time.Second}
	log.Info("MCP streamable HTTP endpoint ready", "url", "http://"+ln.Addr().String()+"/mcp")
	if ready != nil {
		ready <- ln.Addr().String()
	}

	errc := make(chan error, 1)
	go func() { errc <- hs.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return hs.Shutdown(shutdownCtx)
	}
}
