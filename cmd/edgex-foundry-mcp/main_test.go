package main

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex/edgextest"
)

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func noEnv(string) string { return "" }

func TestVersionFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(context.Background(), []string{"--version"}, noEnv, &out, &errOut, nil); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if got := out.String(); got != "edgex-foundry-mcp dev\n" {
		t.Errorf("stdout = %q", got)
	}
}

func TestInvalidConfigExitsNonZero(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(context.Background(), []string{"--max-results", "5000"}, noEnv, &out, &errOut, nil); code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if !strings.Contains(errOut.String(), "1..1024") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestNonLoopbackBindWarns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errOut := &lockedBuffer{}
	ready := make(chan string, 1)
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"--transport", "http", "--http-addr", "0.0.0.0:0"}, noEnv, &bytes.Buffer{}, errOut, ready)
	}()
	select {
	case <-ready:
	case code := <-done:
		t.Fatalf("exited early with %d: %s", code, errOut.String())
	case <-time.After(5 * time.Second):
		t.Fatal("server did not start")
	}
	cancel()
	if code := <-done; code != 0 {
		t.Errorf("exit %d", code)
	}
	if !strings.Contains(errOut.String(), "no client authentication") {
		t.Errorf("missing warning in logs: %s", errOut.String())
	}
}

// TestHTTPTransportEndToEnd starts the real server over streamable HTTP
// against the fake EdgeX and calls a tool through an MCP client.
func TestHTTPTransportEndToEnd(t *testing.T) {
	f := edgextest.New(t)
	env := map[string]string{
		"EDGEX_METADATA_URL": f.Metadata.URL,
		"EDGEX_DATA_URL":     f.Data.URL,
		"EDGEX_COMMAND_URL":  f.Command.URL,
		"EDGEX_TOKEN":        "test-token-value",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errOut := &lockedBuffer{}
	ready := make(chan string, 1)
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"--transport", "http", "--http-addr", "127.0.0.1:0", "--log-level", "debug"},
			func(k string) string { return env[k] }, &bytes.Buffer{}, errOut, ready)
	}()
	var addr string
	select {
	case addr = <-ready:
	case code := <-done:
		t.Fatalf("exited early with %d: %s", code, errOut.String())
	case <-time.After(5 * time.Second):
		t.Fatal("server did not start")
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://" + addr + "/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "system_health", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	_ = cs.Close()
	cancel()
	if code := <-done; code != 0 {
		t.Errorf("exit %d", code)
	}
	logs := errOut.String()
	if strings.Contains(logs, "test-token-value") {
		t.Error("token leaked into logs")
	}
	if !strings.Contains(logs, "read-only mode") {
		t.Error("startup log does not state read-only mode")
	}
}
