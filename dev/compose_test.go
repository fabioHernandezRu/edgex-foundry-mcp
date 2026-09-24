// Package dev holds the development EdgeX stack; its test checks the compose file.
package dev

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestComposePinnedAndLoopbackOnly(t *testing.T) {
	b, err := os.ReadFile("docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	image := regexp.MustCompile(`^\s*image:\s*(\S+)\s*$`)
	port := regexp.MustCompile(`^\s*-\s*"([^"]+)"\s*$`)
	pinned := regexp.MustCompile(`^[a-z0-9./-]+:\d+\.\d+(\.\d+)?(-[a-z0-9.-]+)?$`)
	inPorts := false
	edgexTags := map[string]bool{}
	images, ports := 0, 0
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimSpace(line)
		if m := image.FindStringSubmatch(line); m != nil {
			images++
			if !pinned.MatchString(m[1]) {
				t.Errorf("image not pinned to an exact version: %s", m[1])
			}
			if strings.HasPrefix(m[1], "edgexfoundry/") {
				edgexTags[m[1][strings.LastIndex(m[1], ":")+1:]] = true
			}
		}
		switch {
		case trimmed == "ports:":
			inPorts = true
			continue
		case inPorts && !strings.HasPrefix(trimmed, "-"):
			inPorts = false
		}
		if inPorts {
			if m := port.FindStringSubmatch(line); m != nil {
				ports++
				if !strings.HasPrefix(m[1], "127.0.0.1:") {
					t.Errorf("port not bound to loopback: %s", m[1])
				}
			}
		}
	}
	if images < 8 || ports < 7 {
		t.Errorf("parsed %d images and %d ports; the test parser is out of date", images, ports)
	}
	if len(edgexTags) != 1 {
		t.Errorf("EdgeX images must share one version, got %v", edgexTags)
	}
}
