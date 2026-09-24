// Package edgextest provides an in-process fake of the EdgeX Foundry v4
// core services (REST API v3) for tests. Payloads in testdata/ are modeled on
// the upstream OpenAPI examples and go-mod-core-contracts v4.0.3 DTOs, using
// neutral example names only.
package edgextest

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

//go:embed testdata/*.json
var fixtures embed.FS

// Version is the EdgeX version the fake reports.
const Version = "4.0.2"

// Request records one request received by the fake.
type Request struct {
	Service string // core-metadata, core-data, core-command
	Method  string
	Path    string // escaped path, e.g. /api/v3/device/name/Line%201
	Query   map[string][]string
	Auth    string // Authorization header value
}

// Fake is a set of fake EdgeX core services.
type Fake struct {
	Metadata *httptest.Server
	Data     *httptest.Server
	Command  *httptest.Server
	// Gateway routes /core-metadata, /core-data and /core-command prefixes to
	// the same handlers, like the secure-mode API gateway (without TLS/auth).
	Gateway *httptest.Server

	mu        sync.Mutex
	requests  []Request
	overrides map[string]http.HandlerFunc

	devices       []map[string]any
	services      []map[string]any
	profiles      []map[string]any
	readings      []map[string]any
	coreCommands  []map[string]any
	EventCounts   map[string]int64
	ReadingCounts map[string]int64
}

// New starts a fake EdgeX and registers cleanup on t.
func New(t testing.TB) *Fake {
	t.Helper()
	f := &Fake{
		overrides:     map[string]http.HandlerFunc{},
		EventCounts:   map[string]int64{"Random-Integer-Device": 12},
		ReadingCounts: map[string]int64{"Random-Integer-Device": 48},
	}
	load(t, "devices.json", &f.devices)
	load(t, "deviceservices.json", &f.services)
	load(t, "profiles.json", &f.profiles)
	load(t, "readings.json", &f.readings)
	load(t, "corecommands.json", &f.coreCommands)

	f.Metadata = httptest.NewServer(f.handler("core-metadata", f.metadata))
	f.Data = httptest.NewServer(f.handler("core-data", f.data))
	f.Command = httptest.NewServer(f.handler("core-command", f.command))
	gw := http.NewServeMux()
	for svc, h := range map[string]func(http.ResponseWriter, *http.Request, string){"core-metadata": f.metadata, "core-data": f.data, "core-command": f.command} {
		gw.Handle("/"+svc+"/", http.StripPrefix("/"+svc, f.handler(svc, h)))
	}
	f.Gateway = httptest.NewServer(gw)
	t.Cleanup(func() {
		f.Metadata.Close()
		f.Data.Close()
		f.Command.Close()
		f.Gateway.Close()
	})
	return f
}

func load(t testing.TB, name string, v any) {
	t.Helper()
	b, err := fixtures.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

// Handle overrides the response for one service and escaped path (any method).
func (f *Fake) Handle(service, escapedPath string, h http.HandlerFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.overrides[service+" "+escapedPath] = h
}

// Requests returns a copy of all requests received so far.
func (f *Fake) Requests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.requests)
}

// LastRequest returns the most recent request to a service.
func (f *Fake) LastRequest(service string) (Request, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.requests) - 1; i >= 0; i-- {
		if f.requests[i].Service == service {
			return f.requests[i], true
		}
	}
	return Request{}, false
}

func (f *Fake) handler(service string, route func(http.ResponseWriter, *http.Request, string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.EscapedPath()
		f.mu.Lock()
		f.requests = append(f.requests, Request{Service: service, Method: r.Method, Path: p, Query: r.URL.Query(), Auth: r.Header.Get("Authorization")})
		h := f.overrides[service+" "+p]
		f.mu.Unlock()
		if h != nil {
			h(w, r)
			return
		}
		if r.Method != http.MethodGet {
			WriteError(w, http.StatusMethodNotAllowed, "fake EdgeX only serves GET")
			return
		}
		switch p {
		case "/api/v3/ping":
			writeJSON(w, 200, map[string]any{"apiVersion": "v3", "timestamp": "Tue Sep 24 10:00:00 UTC 2026", "serviceName": service})
			return
		case "/api/v3/version":
			writeJSON(w, 200, map[string]any{"apiVersion": "v3", "version": Version, "serviceName": service})
			return
		}
		route(w, r, p)
	})
}

// segments splits an escaped path under /api/v3/ into unescaped segments.
func segments(p string) []string {
	parts := strings.Split(strings.TrimPrefix(p, "/api/v3/"), "/")
	for i, s := range parts {
		parts[i] = unescape(s)
	}
	return parts
}

func unescape(s string) string {
	if u, err := url.PathUnescape(s); err == nil {
		return u
	}
	return s
}

func (f *Fake) metadata(w http.ResponseWriter, r *http.Request, p string) {
	s := segments(p)
	q := r.URL.Query()
	switch {
	case eq(s, "deviceservice", "all"):
		items := filterLabels(f.services, q.Get("labels"))
		writeList(w, q, "services", items)
	case eq(s, "device", "all"):
		writeList(w, q, "devices", filterLabels(f.devices, q.Get("labels")))
	case len(s) == 4 && s[0] == "device" && s[1] == "service" && s[2] == "name":
		writeList(w, q, "devices", filterField(f.devices, "serviceName", s[3]))
	case len(s) == 4 && s[0] == "device" && s[1] == "profile" && s[2] == "name":
		writeList(w, q, "devices", filterField(f.devices, "profileName", s[3]))
	case len(s) == 3 && s[0] == "device" && s[1] == "name":
		if d := find(f.devices, "name", s[2]); d != nil {
			writeJSON(w, 200, map[string]any{"apiVersion": "v3", "statusCode": 200, "device": d})
			return
		}
		WriteError(w, 404, fmt.Sprintf("no device with name '%s' found", s[2]))
	case eq(s, "deviceprofile", "basicinfo", "all"):
		writeList(w, q, "profiles", basicInfo(filterLabels(f.profiles, q.Get("labels"))))
	case len(s) == 5 && s[0] == "deviceprofile" && s[1] == "manufacturer" && s[3] == "model":
		writeList(w, q, "profiles", filterField(filterField(f.profiles, "manufacturer", s[2]), "model", s[4]))
	case len(s) == 3 && s[0] == "deviceprofile" && s[1] == "manufacturer":
		writeList(w, q, "profiles", filterField(f.profiles, "manufacturer", s[2]))
	case len(s) == 3 && s[0] == "deviceprofile" && s[1] == "model":
		writeList(w, q, "profiles", filterField(f.profiles, "model", s[2]))
	case len(s) == 3 && s[0] == "deviceprofile" && s[1] == "name":
		if pr := find(f.profiles, "name", s[2]); pr != nil {
			writeJSON(w, 200, map[string]any{"apiVersion": "v3", "statusCode": 200, "profile": pr})
			return
		}
		WriteError(w, 404, fmt.Sprintf("no device profile with name '%s' found", s[2]))
	default:
		WriteError(w, 404, "route not found in fake core-metadata: "+p)
	}
}

func (f *Fake) data(w http.ResponseWriter, r *http.Request, p string) {
	s := segments(p)
	q := r.URL.Query()
	switch {
	case len(s) == 5 && s[0] == "event" && s[1] == "count" && s[2] == "device":
		writeJSON(w, 200, map[string]any{"apiVersion": "v3", "statusCode": 200, "count": f.EventCounts[s[4]]})
	case len(s) == 5 && s[0] == "reading" && s[1] == "count" && s[2] == "device":
		writeJSON(w, 200, map[string]any{"apiVersion": "v3", "statusCode": 200, "count": f.ReadingCounts[s[4]]})
	case len(s) >= 4 && s[0] == "reading" && s[1] == "device" && s[2] == "name":
		device, rest := s[3], s[4:]
		var resource string
		var hasRange bool
		var start, end int64
		if len(rest) >= 2 && rest[0] == "resourceName" {
			resource, rest = rest[1], rest[2:]
		}
		if len(rest) == 4 && rest[0] == "start" && rest[2] == "end" {
			var err1, err2 error
			start, err1 = strconv.ParseInt(rest[1], 10, 64)
			end, err2 = strconv.ParseInt(rest[3], 10, 64)
			if err1 != nil || err2 != nil || end < start {
				WriteError(w, 400, "invalid start/end")
				return
			}
			hasRange, rest = true, nil
		}
		if len(rest) != 0 {
			WriteError(w, 404, "route not found in fake core-data: "+p)
			return
		}
		items := f.deviceReadings(device)
		var out []map[string]any
		for _, rd := range items {
			if resource != "" && rd["resourceName"] != resource {
				continue
			}
			if hasRange {
				o := int64(rd["origin"].(float64))
				if o < start || o > end {
					continue
				}
			}
			out = append(out, rd)
		}
		writeList(w, q, "readings", out)
	default:
		WriteError(w, 404, "route not found in fake core-data: "+p)
	}
}

func (f *Fake) deviceReadings(device string) []map[string]any {
	if device == "Random-Binary-Device" {
		payload := make([]byte, 40*1024)
		return []map[string]any{{
			"id": "r-bin", "origin": float64(1727172000000000000), "deviceName": device, "resourceName": "Binary",
			"profileName": device, "valueType": "Binary", "mediaType": "image/jpeg",
			"binaryValue": base64.StdEncoding.EncodeToString(payload),
		}}
	}
	return filterField(f.readings, "deviceName", device)
}

func (f *Fake) command(w http.ResponseWriter, r *http.Request, p string) {
	s := segments(p)
	q := r.URL.Query()
	switch {
	case eq(s, "device", "all"):
		writeList(w, q, "deviceCoreCommands", f.coreCommands)
	case len(s) == 3 && s[0] == "device" && s[1] == "name":
		if d := find(f.coreCommands, "deviceName", s[2]); d != nil {
			writeJSON(w, 200, map[string]any{"apiVersion": "v3", "statusCode": 200, "deviceCoreCommand": d})
			return
		}
		WriteError(w, 404, fmt.Sprintf("fail to query device by name %s", s[2]))
	case len(s) == 4 && s[0] == "device" && s[1] == "name":
		device, cmd := s[2], s[3]
		if d := find(f.devices, "name", device); d != nil && d["adminState"] == "LOCKED" {
			WriteError(w, 423, fmt.Sprintf("request failed, status code: 423, err: device %s locked", device))
			return
		}
		cc := find(f.coreCommands, "deviceName", device)
		if cc == nil {
			WriteError(w, 404, "device not found")
			return
		}
		var readings []map[string]any
		for _, rd := range f.readings {
			if rd["deviceName"] == device && rd["resourceName"] == cmd {
				readings = append(readings, rd)
				break
			}
		}
		if readings == nil {
			WriteError(w, 404, fmt.Sprintf("command %s not found", cmd))
			return
		}
		writeJSON(w, 200, map[string]any{"apiVersion": "v3", "statusCode": 200, "event": map[string]any{
			"apiVersion": "v3", "id": "e-0001", "deviceName": device, "profileName": cc["profileName"],
			"sourceName": cmd, "origin": readings[0]["origin"], "readings": readings,
		}})
	default:
		WriteError(w, 404, "route not found in fake core-command: "+p)
	}
}

// WriteError writes an EdgeX BaseResponse error body.
func WriteError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"apiVersion": "v3", "message": msg, "statusCode": status})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeList(w http.ResponseWriter, q map[string][]string, key string, items []map[string]any) {
	get := func(k string, def int) int {
		if v, ok := q[k]; ok && len(v) > 0 {
			if n, err := strconv.Atoi(v[0]); err == nil {
				return n
			}
		}
		return def
	}
	offset, limit := get("offset", 0), get("limit", 20)
	if limit < 0 || limit > 1024 {
		WriteError(w, 400, fmt.Sprintf("querystring limit's value %d is out of min -1 ~ max 1024 range.", limit))
		return
	}
	total := len(items)
	if offset > total && total > 0 {
		WriteError(w, 416, "query objects bounds out of range")
		return
	}
	end := min(offset+limit, total)
	page := []map[string]any{}
	if offset < total {
		page = items[offset:end]
	}
	writeJSON(w, 200, map[string]any{"apiVersion": "v3", "statusCode": 200, "totalCount": total, key: page})
}

func eq(s []string, want ...string) bool { return slices.Equal(s, want) }

func find(items []map[string]any, field, value string) map[string]any {
	for _, it := range items {
		if it[field] == value {
			return it
		}
	}
	return nil
}

func filterField(items []map[string]any, field, value string) []map[string]any {
	var out []map[string]any
	for _, it := range items {
		if it[field] == value {
			out = append(out, it)
		}
	}
	return out
}

// filterLabels keeps items carrying all given labels (EdgeX uses AND semantics).
func filterLabels(items []map[string]any, csv string) []map[string]any {
	if csv == "" {
		return items
	}
	want := strings.Split(csv, ",")
	var out []map[string]any
	for _, it := range items {
		have, _ := it["labels"].([]any)
		ok := true
		for _, l := range want {
			if !slices.Contains(have, any(l)) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, it)
		}
	}
	return out
}

func basicInfo(profiles []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		b := map[string]any{}
		for _, k := range []string{"created", "modified", "id", "name", "manufacturer", "description", "model", "labels", "linkedDeviceCount"} {
			if v, ok := p[k]; ok {
				b[k] = v
			}
		}
		out = append(out, b)
	}
	return out
}
