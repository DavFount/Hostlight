package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
)

const maxHTTPBodyBytes = 1024 * 1024

func normalizeHTTPBodyFields(t Target) Target {
	if t.Type != "http" {
		return t
	}
	switch t.BodyMode {
	case "":
		t.BodyExpected = ""
		t.JSONPointer = ""
		t.JSONType = ""
	case "contains":
		t.JSONPointer = ""
		t.JSONType = ""
	case "json":
		if t.JSONType == "" {
			t.JSONType = "string"
		}
		if t.JSONType == "null" {
			t.BodyExpected = ""
		}
	}
	return t
}

func boundedJSONNumber(value json.Number) (*big.Rat, bool) {
	text := string(value)
	if len(text) > 4096 {
		return nil, false
	}
	if index := strings.IndexAny(text, "eE"); index >= 0 {
		exponent, err := strconv.ParseInt(text[index+1:], 10, 32)
		if err != nil || exponent > 4096 || exponent < -4096 {
			return nil, false
		}
	}
	return new(big.Rat).SetString(text)
}

func decodeJSONValue(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("Expected a single JSON value")
	}
	return value, nil
}
func pointerParts(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if len(pointer) > 1024 || !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("JSON field path must start with / (for example /status), up to 1,024 characters")
	}
	parts := strings.Split(pointer[1:], "/")
	if len(parts) > 32 {
		return nil, fmt.Errorf("JSON field path may contain at most 32 levels")
	}
	for i, part := range parts {
		for j := 0; j < len(part); j++ {
			if part[j] == '~' {
				if j+1 == len(part) || (part[j+1] != '0' && part[j+1] != '1') {
					return nil, fmt.Errorf("Escape ~ as ~0 and / as ~1 in JSON field names")
				}
				j++
			}
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts, nil
}
func expectedJSONValue(t Target) (any, error) {
	switch t.JSONType {
	case "", "string":
		return t.BodyExpected, nil
	case "number":
		value, err := decodeJSONValue([]byte(t.BodyExpected))
		if err != nil {
			return nil, fmt.Errorf("Enter a valid JSON number")
		}
		number, ok := value.(json.Number)
		if !ok {
			return nil, fmt.Errorf("Enter a valid JSON number")
		}
		if _, ok = boundedJSONNumber(number); !ok {
			return nil, fmt.Errorf("JSON numbers support up to 4,096 characters and exponents from -4096 to 4096")
		}
		return value, nil
	case "boolean":
		if t.BodyExpected == "true" {
			return true, nil
		}
		if t.BodyExpected == "false" {
			return false, nil
		}
		return nil, fmt.Errorf("Choose true or false for the expected boolean")
	case "null":
		return nil, nil
	default:
		return nil, fmt.Errorf("Choose text, number, boolean, or null as the JSON value type")
	}
}
func validateHTTPBodyRule(t Target) error {
	if len(t.BodyExpected) > 4096 {
		return fmt.Errorf("Expected response value must be at most 4,096 bytes")
	}
	switch t.BodyMode {
	case "":
		return nil
	case "contains":
		if t.BodyExpected == "" {
			return fmt.Errorf("Enter text that must appear in the response")
		}
		return nil
	case "json":
		if _, err := pointerParts(t.JSONPointer); err != nil {
			return err
		}
		_, err := expectedJSONValue(t)
		return err
	default:
		return fmt.Errorf("Choose status only, response contains text, or JSON field equals")
	}
}
func matchHTTPBody(t Target, data []byte) error {
	if t.BodyMode == "contains" {
		if strings.Contains(string(data), t.BodyExpected) {
			return nil
		}
		return fmt.Errorf("HTTP response did not contain the expected text")
	}
	value, err := decodeJSONValue(data)
	if err != nil {
		return fmt.Errorf("HTTP response was not a single valid JSON document")
	}
	parts, err := pointerParts(t.JSONPointer)
	if err != nil {
		return err
	}
	for _, part := range parts {
		switch current := value.(type) {
		case map[string]any:
			var exists bool
			value, exists = current[part]
			if !exists {
				return fmt.Errorf("Expected JSON field was missing")
			}
		case []any:
			index, e := strconv.Atoi(part)
			if e != nil || index < 0 || index >= len(current) || strconv.Itoa(index) != part {
				return fmt.Errorf("Expected JSON array entry was missing")
			}
			value = current[index]
		default:
			return fmt.Errorf("Expected JSON field was missing")
		}
	}
	expected, err := expectedJSONValue(t)
	if err != nil {
		return err
	}
	matched := false
	switch want := expected.(type) {
	case nil:
		matched = value == nil
	case string:
		got, ok := value.(string)
		matched = ok && got == want
	case bool:
		got, ok := value.(bool)
		matched = ok && got == want
	case json.Number:
		if got, ok := value.(json.Number); ok {
			left, lok := boundedJSONNumber(got)
			right, rok := boundedJSONNumber(want)
			matched = lok && rok && left.Cmp(right) == 0
		}
	}
	if !matched {
		return fmt.Errorf("JSON field did not match the expected value and type")
	}
	return nil
}
