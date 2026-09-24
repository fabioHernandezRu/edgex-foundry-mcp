// Package scripts holds repository-level checks for release metadata.
package scripts

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

type serverJSON struct {
	Schema      string `json:"$schema"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Repository  struct {
		URL    string `json:"url"`
		Source string `json:"source"`
	} `json:"repository"`
	Packages []struct {
		RegistryType string `json:"registryType"`
		Identifier   string `json:"identifier"`
		Transport    struct {
			Type string `json:"type"`
		} `json:"transport"`
		EnvironmentVariables []struct {
			Name     string `json:"name"`
			IsSecret bool   `json:"isSecret"`
		} `json:"environmentVariables"`
	} `json:"packages"`
}

// TestServerJSONMatchesRelease keeps the MCP registry metadata consistent with
// the GoReleaser image configuration.
func TestServerJSONMatchesRelease(t *testing.T) {
	b, err := os.ReadFile("../server.json")
	if err != nil {
		t.Fatal(err)
	}
	var s serverJSON
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	gr, err := os.ReadFile("../.goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(gr)

	if s.Schema != "https://static.modelcontextprotocol.io/schemas/2025-12-11/server.schema.json" {
		t.Errorf("unexpected schema %q", s.Schema)
	}
	if !regexp.MustCompile(`^io\.github\.[A-Za-z0-9-]+/edgex-foundry-mcp$`).MatchString(s.Name) {
		t.Errorf("name %q must be io.github.<login>/edgex-foundry-mcp", s.Name)
	}
	if !strings.Contains(cfg, `io.modelcontextprotocol.server.name: "`+s.Name+`"`) {
		t.Errorf("OCI label in .goreleaser.yaml must equal server name %q", s.Name)
	}
	if len(s.Description) == 0 || len(s.Description) > 100 {
		t.Errorf("description must be 1..100 characters, got %d", len(s.Description))
	}
	if s.Repository.Source != "github" || !strings.HasSuffix(s.Repository.URL, "/edgex-foundry-mcp") {
		t.Errorf("repository = %+v", s.Repository)
	}
	if len(s.Packages) != 1 {
		t.Fatalf("expected one package, got %d", len(s.Packages))
	}
	p := s.Packages[0]
	image := regexp.MustCompile(`"(ghcr\.io/[a-z0-9./-]+)"`).FindStringSubmatch(cfg)
	if p.RegistryType != "oci" || p.Transport.Type != "stdio" || image == nil || !strings.HasPrefix(p.Identifier, image[1]+":") {
		t.Errorf("package %+v does not match image %v", p, image)
	}
	if p.Identifier != strings.ToLower(p.Identifier) {
		t.Error("OCI identifiers must be lowercase")
	}
	secret := false
	for _, ev := range p.EnvironmentVariables {
		if ev.Name == "EDGEX_TOKEN" {
			secret = ev.IsSecret
		}
	}
	if !secret {
		t.Error("EDGEX_TOKEN must be marked isSecret")
	}
}
