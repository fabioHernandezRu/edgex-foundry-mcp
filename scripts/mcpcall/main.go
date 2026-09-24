// Command mcpcall is a tiny MCP client for live checks of edgex-foundry-mcp.
//
//	go run ./scripts/mcpcall -- tools/list
//	go run ./scripts/mcpcall -- call get_latest_readings '{"device":"Random-Integer-Device","limit":3}'
//	go run ./scripts/mcpcall -- read edgex://deviceprofile/Random-Integer-Device
//	go run ./scripts/mcpcall -url http://127.0.0.1:8080/mcp -- tools/list
//
// By default it starts ./bin/edgex-foundry-mcp over stdio; the server inherits
// the environment, so EDGEX_* variables apply. Extra server flags can be passed
// with -server-args.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	serverPath := flag.String("server", "./bin/edgex-foundry-mcp", "server binary to start over stdio")
	serverArgs := flag.String("server-args", "", "space-separated extra flags for the server")
	endpoint := flag.String("url", "", "streamable HTTP endpoint instead of stdio, e.g. http://127.0.0.1:8080/mcp")
	timeout := flag.Duration("timeout", 60*time.Second, "overall timeout")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: mcpcall [flags] -- tools/list | call <tool> [json-args] | read <uri>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if err := run(*serverPath, *serverArgs, *endpoint, *timeout, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "mcpcall:", err)
		os.Exit(1)
	}
}

func run(serverPath, serverArgs, endpoint string, timeout time.Duration, args []string) error {
	if len(args) == 0 {
		flag.Usage()
		return fmt.Errorf("missing command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var transport mcp.Transport
	if endpoint != "" {
		transport = &mcp.StreamableClientTransport{Endpoint: endpoint}
	} else {
		cmd := exec.CommandContext(ctx, serverPath, strings.Fields(serverArgs)...) // #nosec G204 -- developer tool
		cmd.Stderr = os.Stderr
		transport = &mcp.CommandTransport{Command: cmd}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "mcpcall", Version: "dev"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return err
	}
	defer func() { _ = cs.Close() }()

	switch args[0] {
	case "tools/list":
		res, err := cs.ListTools(ctx, nil)
		if err != nil {
			return err
		}
		for _, t := range res.Tools {
			ro := t.Annotations != nil && t.Annotations.ReadOnlyHint
			fmt.Printf("%-24s readOnly=%-5v %s\n", t.Name, ro, firstSentence(t.Description))
		}
	case "call":
		if len(args) < 2 {
			return fmt.Errorf("usage: call <tool> [json-args]")
		}
		arguments := map[string]any{}
		if len(args) > 2 {
			if err := json.Unmarshal([]byte(args[2]), &arguments); err != nil {
				return fmt.Errorf("arguments must be a JSON object: %w", err)
			}
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: args[1], Arguments: arguments})
		if err != nil {
			return err
		}
		if res.IsError {
			for _, c := range res.Content {
				if tc, ok := c.(*mcp.TextContent); ok {
					fmt.Println("TOOL ERROR:", tc.Text)
				}
			}
			return fmt.Errorf("tool returned an error")
		}
		return printJSON(res.StructuredContent)
	case "read":
		if len(args) < 2 {
			return fmt.Errorf("usage: read <uri>")
		}
		res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: args[1]})
		if err != nil {
			return err
		}
		for _, c := range res.Contents {
			fmt.Println(c.Text)
		}
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func firstSentence(s string) string {
	if i := strings.Index(s, ". "); i > 0 {
		return s[:i+1]
	}
	return s
}
