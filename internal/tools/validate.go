package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// validateSetValue checks a SET value (always sent as a string) against the
// EdgeX valueType and, for numeric values, the profile's minimum/maximum.
// It mirrors what device-sdk-go v4 accepts so that errors surface before any
// request reaches the device (every failed SET counts toward the device's
// failure tracking). Error messages never quote the value.
func validateSetValue(valueType, v string, minimum, maximum *float64) error {
	switch {
	case valueType == "String":
		return nil
	case valueType == "Binary":
		return errors.New("values of type Binary cannot be set through the EdgeX REST API")
	case strings.TrimSpace(v) == "":
		return fmt.Errorf("empty value is not valid for %s", valueType)
	case valueType == "Object":
		var obj map[string]any
		if err := json.Unmarshal([]byte(v), &obj); err != nil || obj == nil {
			return errors.New("values of type Object must be a JSON object")
		}
		return nil
	case strings.HasSuffix(valueType, "Array"):
		return validateArray(strings.TrimSuffix(valueType, "Array"), v, minimum, maximum)
	}

	switch valueType {
	case "Bool":
		if _, err := strconv.ParseBool(v); err != nil {
			return errors.New("values of type Bool must be true or false")
		}
		return nil
	case "Int8", "Int16", "Int32", "Int64":
		bits, _ := strconv.Atoi(strings.TrimPrefix(valueType, "Int"))
		n, err := strconv.ParseInt(v, 10, bits)
		if err != nil {
			return fmt.Errorf("not a valid %s (integer in the %d-bit signed range)", valueType, bits)
		}
		return checkBounds(new(big.Float).SetInt64(n), minimum, maximum)
	case "Uint8", "Uint16", "Uint32", "Uint64":
		bits, _ := strconv.Atoi(strings.TrimPrefix(valueType, "Uint"))
		n, err := strconv.ParseUint(v, 10, bits)
		if err != nil {
			return fmt.Errorf("not a valid %s (integer in the %d-bit unsigned range)", valueType, bits)
		}
		return checkBounds(new(big.Float).SetUint64(n), minimum, maximum)
	case "Float32", "Float64":
		bits, _ := strconv.Atoi(strings.TrimPrefix(valueType, "Float"))
		f, err := strconv.ParseFloat(v, bits)
		if err != nil {
			return fmt.Errorf("not a valid %s", valueType)
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("NaN and infinite values are not allowed for %s", valueType)
		}
		return checkBounds(big.NewFloat(f), minimum, maximum)
	default:
		return fmt.Errorf("unsupported valueType %q", valueType)
	}
}

// checkBounds compares exactly (no float64 rounding for large integers).
func checkBounds(n *big.Float, minimum, maximum *float64) error {
	if minimum != nil && n.Cmp(big.NewFloat(*minimum)) < 0 {
		return fmt.Errorf("below the minimum %v", *minimum)
	}
	if maximum != nil && n.Cmp(big.NewFloat(*maximum)) > 0 {
		return fmt.Errorf("above the maximum %v", *maximum)
	}
	return nil
}

// validateArray validates JSON array text element by element.
func validateArray(base, v string, minimum, maximum *float64) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(v)))
	dec.UseNumber()
	var elems []any
	if err := dec.Decode(&elems); err != nil || elems == nil || dec.More() {
		return fmt.Errorf("%sArray values must be JSON array text such as [1,2,3]", base)
	}
	for i, e := range elems {
		var err error
		switch base {
		case "String":
			if _, ok := e.(string); !ok {
				err = errors.New("must be a string")
			}
		case "Bool":
			if _, ok := e.(bool); !ok {
				err = errors.New("must be true or false")
			}
		case "Object":
			if _, ok := e.(map[string]any); !ok {
				err = errors.New("must be a JSON object")
			}
		case "Int8", "Int16", "Int32", "Int64", "Uint8", "Uint16", "Uint32", "Uint64", "Float32", "Float64":
			num, ok := e.(json.Number)
			if !ok {
				err = errors.New("must be a number")
			} else {
				err = validateSetValue(base, num.String(), minimum, maximum)
			}
		default:
			return fmt.Errorf("unsupported valueType %sArray", base)
		}
		if err != nil {
			return fmt.Errorf("element %d: %w", i, err)
		}
	}
	return nil
}
