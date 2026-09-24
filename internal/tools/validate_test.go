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
		{"Int64", "9007199254740993", nil, f(9007199254740992), false},
		{"Int64", "9007199254740992", nil, f(9007199254740992), true},
		{"Uint8", "255", nil, nil, true},
		{"Uint8", "-1", nil, nil, false},
		{"Uint64", "18446744073709551615", nil, nil, true},
		{"Float32", "3.5", nil, nil, true},
		{"Float64", "1e3", nil, f(999), false},
		{"Float64", "abc", nil, nil, false},
		{"Float64", "NaN", f(0), f(10), false},
		{"Float64", "Inf", nil, nil, false},
		{"Float32", "-infinity", nil, nil, false},
		{"Bool", "true", nil, nil, true},
		{"Bool", "yes", nil, nil, false},
		{"String", "", nil, nil, true},
		{"Int8Array", "[1,2,42]", nil, nil, true},
		{"Int8Array", "1,2", nil, nil, false},
		{"Int8Array", "[1000]", nil, nil, false},
		{"Int8Array", `["a"]`, nil, nil, false},
		{"Int8Array", "[50,101]", nil, f(100), false},
		{"Float64Array", "[1.5,2]", nil, nil, true},
		{"BoolArray", "[true,false]", nil, nil, true},
		{"BoolArray", `["true"]`, nil, nil, false},
		{"StringArray", `["a","b"]`, nil, nil, true},
		{"StringArray", `[1]`, nil, nil, false},
		{"ObjectArray", `[{"a":1}]`, nil, nil, true},
		{"ObjectArray", `[1]`, nil, nil, false},
		{"FooArray", "[1]", nil, nil, false},
		{"Int8Array", "[1] [2]", nil, nil, false},
		{"Object", `{"a":1}`, nil, nil, true},
		{"Object", `{a:1}`, nil, nil, false},
		{"Object", `1`, nil, nil, false},
		{"Object", `[1]`, nil, nil, false},
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
