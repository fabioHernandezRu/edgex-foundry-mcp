package tools

import (
	"reflect"
	"strings"
	"testing"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

func TestRedactMap(t *testing.T) {
	in := map[string]any{
		"Host":     "192.0.2.10",
		"Password": "example-pass",
		"nested": map[string]any{
			"ApiKey": "k", "list": []any{map[string]any{"clientSecret": "s", "port": "1"}},
		},
		"AuthMode": "usernamepassword",
		"Port":     "1883",
	}
	got := RedactMap(in)
	want := map[string]any{
		"Host":     "192.0.2.10",
		"Password": Redacted,
		"nested": map[string]any{
			"ApiKey": Redacted, "list": []any{map[string]any{"clientSecret": Redacted, "port": "1"}},
		},
		"AuthMode": Redacted, // over-redaction is accepted
		"Port":     "1883",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	if in["Password"] != "example-pass" {
		t.Error("input was modified")
	}
	if RedactMap(nil) != nil || RedactProtocols(nil) != nil {
		t.Error("nil input must give nil")
	}
}

func TestSensitiveKeys(t *testing.T) {
	for _, k := range []string{"password", "PASSWD", "pwd", "SecretName", "token", "apikey", "api_key", "PrivateKey", "Credentials", "Authorization", "certFile"} {
		if !IsSensitiveKey(k) {
			t.Errorf("%s should be sensitive", k)
		}
	}
	for _, k := range []string{"Host", "Port", "Address", "Topic", "UnitID", "Username"} {
		if IsSensitiveKey(k) {
			t.Errorf("%s should not be sensitive", k)
		}
	}
}

func TestShapeReading(t *testing.T) {
	long := strings.Repeat("a", 2000)
	short := "42"
	tests := []struct {
		name  string
		in    edgex.Reading
		check func(ReadingOut) bool
	}{
		{"simple", edgex.Reading{ResourceName: "Int8", Origin: 1727172003000000000, ValueType: "Int8", Value: &short},
			func(o ReadingOut) bool { return *o.Value == "42" && o.Time == "2024-09-24T10:00:03Z" }},
		{"long string truncated", edgex.Reading{ValueType: "String", Value: &long},
			func(o ReadingOut) bool { return len(*o.Value) == maxStringBytes && o.Truncated }},
		{"binary summarized", edgex.Reading{ValueType: "Binary", BinaryValue: make([]byte, 10), MediaType: "image/png"},
			func(o ReadingOut) bool { return o.BinarySize == 10 && o.Value == nil && o.MediaType == "image/png" }},
		{"object", edgex.Reading{ValueType: "Object", ObjectValue: []byte(`{"a":1}`)},
			func(o ReadingOut) bool { m, ok := o.ObjectValue.(map[string]any); return ok && m["a"] == float64(1) }},
		{"large object omitted", edgex.Reading{ValueType: "Object", ObjectValue: []byte(`"` + strings.Repeat("x", maxObjectBytes) + `"`)},
			func(o ReadingOut) bool { return o.ObjectValue == nil && o.Truncated }},
		{"null value", edgex.Reading{ValueType: "Int8"},
			func(o ReadingOut) bool { return o.Value == nil && !o.Truncated }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if o := shapeReading(tt.in); !tt.check(o) {
				t.Errorf("unexpected %+v", o)
			}
		})
	}
}

func TestListInfo(t *testing.T) {
	li := listInfo(10, 0, 4)
	if li.NextOffset == nil || *li.NextOffset != 4 {
		t.Errorf("%+v", li)
	}
	if li := listInfo(4, 0, 4); li.NextOffset != nil {
		t.Errorf("%+v", li)
	}
	if li := listInfo(0, 0, 0); li.NextOffset != nil {
		t.Errorf("%+v", li)
	}
}

func TestTimeConversion(t *testing.T) {
	if nsTime(0) != "" || msTime(0) != "" {
		t.Error("zero must be empty")
	}
	if got := msTime(1727000000000); got != "2024-09-22T10:13:20Z" {
		t.Errorf("msTime = %s", got)
	}
}
