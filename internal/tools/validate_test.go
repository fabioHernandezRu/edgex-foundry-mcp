package tools

import "testing"

func TestValidateSetValue(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	tests := []struct {
		valueType, v string
		min, max     *float64
		ok           bool
	}{
		{"Int8", "42", nil, nil, true},
		{"Int8", "127", nil, nil, true},
		{"Int8", "128", nil, nil, false},
		{"Int8", "-129", nil, nil, false},
		{"Int8", "4.2", nil, nil, false},
		{"Int8", "", nil, nil, false},
		{"Int8", "200", f(-100), f(100), false},
		{"Int8", "-101", f(-100), f(100), false},
		{"Int8", "100", f(-100), f(100), true},
		{"Int64", "9223372036854775807", nil, nil, true},
		{"Uint8", "255", nil, nil, true},
		{"Uint8", "-1", nil, nil, false},
		{"Uint64", "18446744073709551615", nil, nil, true},
		{"Float32", "3.5", nil, nil, true},
		{"Float64", "1e3", nil, f(999), false},
		{"Float64", "abc", nil, nil, false},
		{"Bool", "true", nil, nil, true},
		{"Bool", "yes", nil, nil, false},
		{"String", "", nil, nil, true},
		{"Int8Array", "[1,2,42]", nil, nil, true},
		{"Int8Array", "1,2", nil, nil, false},
		{"Object", `{"a":1}`, nil, nil, true},
		{"Object", `{a:1}`, nil, nil, false},
		{"Binary", "AAAA", nil, nil, false},
		{"Weird", "1", nil, nil, false},
	}
	for _, tt := range tests {
		err := validateSetValue(tt.valueType, tt.v, tt.min, tt.max)
		if (err == nil) != tt.ok {
			t.Errorf("%s %q: err = %v, want ok=%v", tt.valueType, tt.v, err, tt.ok)
		}
	}
}
