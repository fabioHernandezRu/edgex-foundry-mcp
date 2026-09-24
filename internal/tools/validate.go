package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// validateSetValue checks a SET value (always sent as a string) against the
// EdgeX valueType and, for numeric scalars, the profile's minimum/maximum.
// It mirrors what device-sdk-go v4 accepts so errors surface before any
// request reaches the device.
func validateSetValue(valueType, v string, minimum, maximum *float64) error {
	switch {
	case valueType == "String":
		return nil
	case valueType == "Binary":
		return errors.New("values of type Binary cannot be set through the EdgeX REST API")
	case strings.TrimSpace(v) == "":
		return fmt.Errorf("empty value is not valid for %s", valueType)
	case valueType == "Object":
		if !json.Valid([]byte(v)) {
			return errors.New("values of type Object must be valid JSON")
		}
		return nil
	case strings.HasSuffix(valueType, "Array"):
		var arr []any
		if err := json.Unmarshal([]byte(v), &arr); err != nil {
			return fmt.Errorf("%s values must be JSON array text such as [1,2,3]", valueType)
		}
		return nil
	}

	var num float64
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
			return fmt.Errorf("%q is not a valid %s (integer in %d-bit signed range)", v, valueType, bits)
		}
		num = float64(n)
	case "Uint8", "Uint16", "Uint32", "Uint64":
		bits, _ := strconv.Atoi(strings.TrimPrefix(valueType, "Uint"))
		n, err := strconv.ParseUint(v, 10, bits)
		if err != nil {
			return fmt.Errorf("%q is not a valid %s (integer in %d-bit unsigned range)", v, valueType, bits)
		}
		num = float64(n)
	case "Float32", "Float64":
		bits, _ := strconv.Atoi(strings.TrimPrefix(valueType, "Float"))
		f, err := strconv.ParseFloat(v, bits)
		if err != nil {
			return fmt.Errorf("%q is not a valid %s", v, valueType)
		}
		num = f
	default:
		return fmt.Errorf("unsupported valueType %q", valueType)
	}
	if minimum != nil && num < *minimum {
		return fmt.Errorf("%s is below the minimum %v", v, *minimum)
	}
	if maximum != nil && num > *maximum {
		return fmt.Errorf("%s is above the maximum %v", v, *maximum)
	}
	return nil
}
