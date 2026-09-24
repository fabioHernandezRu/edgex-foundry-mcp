package tools

import (
	"encoding/json"
	"time"

	"github.com/fabioHernandezRu/edgex-foundry-mcp/internal/edgex"
)

const (
	// maxStringBytes bounds any single reading value returned inline.
	maxStringBytes = 1024
	// maxObjectBytes bounds an inline object reading value.
	maxObjectBytes = 4096
)

// ListInfo is the pagination envelope of every list tool.
type ListInfo struct {
	TotalCount int    `json:"totalCount" jsonschema:"total number of matching items reported by EdgeX"`
	Offset     int    `json:"offset" jsonschema:"offset of the first returned item"`
	Count      int    `json:"count" jsonschema:"number of items returned"`
	NextOffset *int   `json:"nextOffset,omitempty" jsonschema:"offset to request the next page, present only when more items exist"`
	Hint       string `json:"hint,omitempty" jsonschema:"guidance when the result is empty or truncated"`
}

func listInfo(total, offset, count int) ListInfo {
	if offset < 0 {
		offset = 0
	}
	li := ListInfo{TotalCount: total, Offset: offset, Count: count}
	if count > 0 && total > offset+count {
		n := offset + count
		li.NextOffset = &n
	}
	return li
}

// nsTime renders EdgeX nanosecond timestamps (reading/event origin) as RFC 3339 UTC.
func nsTime(ns int64) string {
	if ns <= 0 {
		return ""
	}
	return time.Unix(0, ns).UTC().Format(time.RFC3339Nano)
}

// msTime renders EdgeX millisecond timestamps (created/modified) as RFC 3339 UTC.
func msTime(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

// ReadingOut is the compact, LLM-friendly form of an EdgeX reading.
type ReadingOut struct {
	Resource    string  `json:"resource" jsonschema:"device resource name"`
	Time        string  `json:"time" jsonschema:"reading origin time, RFC 3339 UTC"`
	ValueType   string  `json:"valueType" jsonschema:"EdgeX value type, e.g. Int8, Float64, Bool, Binary, Object"`
	Value       *string `json:"value,omitempty" jsonschema:"value as a string (numbers and booleans are encoded as strings by EdgeX)"`
	Units       string  `json:"units,omitempty" jsonschema:"units, when the profile defines them"`
	MediaType   string  `json:"mediaType,omitempty" jsonschema:"media type of a binary value"`
	BinarySize  int     `json:"binarySize,omitempty" jsonschema:"size in bytes of a binary value, which is never returned inline"`
	ObjectValue any     `json:"objectValue,omitempty" jsonschema:"JSON value of an Object reading"`
	Truncated   bool    `json:"truncated,omitempty" jsonschema:"true when the value was truncated or omitted because it was too large"`
}

func shapeReading(r edgex.Reading) ReadingOut {
	out := ReadingOut{
		Resource:  r.ResourceName,
		Time:      nsTime(r.Origin),
		ValueType: r.ValueType,
		Units:     r.Units,
		MediaType: r.MediaType,
	}
	switch {
	case len(r.BinaryValue) > 0 || r.ValueType == "Binary":
		out.BinarySize = len(r.BinaryValue)
	case len(r.ObjectValue) > 0 && string(r.ObjectValue) != "null":
		if len(r.ObjectValue) > maxObjectBytes {
			out.Truncated = true
			break
		}
		var v any
		if err := json.Unmarshal(r.ObjectValue, &v); err == nil {
			out.ObjectValue = v
		}
	case r.Value != nil:
		v := *r.Value
		if len(v) > maxStringBytes {
			v = v[:maxStringBytes]
			out.Truncated = true
		}
		out.Value = &v
	}
	return out
}

func shapeReadings(rs []edgex.Reading) []ReadingOut {
	out := make([]ReadingOut, 0, len(rs))
	for _, r := range rs {
		out = append(out, shapeReading(r))
	}
	return out
}
