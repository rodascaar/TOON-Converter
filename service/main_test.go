package main

import (
	"encoding/json"
	"testing"
)

func TestTOONEncoder_SimpleObject(t *testing.T) {
	input := map[string]interface{}{
		"id":   float64(123),
		"name": "Alice",
	}

	encoder := NewTOONEncoder()
	result := encoder.Encode(input)

	expected := "id: 123\nname: Alice"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestTOONEncoder_TabularArray(t *testing.T) {
	input := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"id": float64(1), "name": "Alice"},
			map[string]interface{}{"id": float64(2), "name": "Bob"},
		},
	}

	encoder := NewTOONEncoder()
	result := encoder.Encode(input)

	expected := "users[2]{id,name}:\n    1,Alice\n    2,Bob"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestTOONEncoder_TabDelimiter(t *testing.T) {
	input := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": float64(1), "name": "Widget"},
			map[string]interface{}{"id": float64(2), "name": "Gadget"},
		},
	}

	opts := TOONOptions{
		Delimiter: "\t",
	}
	encoder, _ := NewTOONEncoderWithOptions(opts)
	result := encoder.Encode(input)

	expected := "items[2 ]{id name}:\n    1\tWidget\n    2\tGadget"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestTOONEncoder_LengthMarker(t *testing.T) {
	input := map[string]interface{}{
		"tags": []interface{}{"foo", "bar", "baz"},
	}

	opts := TOONOptions{
		LengthMarker: true,
	}
	encoder, _ := NewTOONEncoderWithOptions(opts)
	result := encoder.Encode(input)

	expected := "tags[#3]: foo,bar,baz"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestTOONEncoder_StringQuoting(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", `""`},
		{"normal", "hello", "hello"},
		{"with comma", "hello,world", `"hello,world"`},
		{"with colon", "key:value", `"key:value"`},
		{"looks like bool", "true", `"true"`},
		{"looks like number", "123", `"123"`},
		{"with leading space", " padded", `" padded"`},
		{"with dash prefix", "- item", `"- item"`},
	}

	encoder := NewTOONEncoder()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encoder.encodeString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestTOONEncoder_NestedArrays(t *testing.T) {
	input := map[string]interface{}{
		"matrix": []interface{}{
			[]interface{}{float64(1), float64(2)},
			[]interface{}{float64(3), float64(4)},
		},
	}

	encoder := NewTOONEncoder()
	result := encoder.Encode(input)

	expected := "matrix[2]:\n    - [2]: 1,2\n    - [2]: 3,4"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestTOONEncoder_ComplexNested(t *testing.T) {
	jsonStr := `{
		"users": [
			{"id": 1, "name": "Alice", "active": true},
			{"id": 2, "name": "Bob", "active": false}
		],
		"metadata": {
			"total": 2,
			"page": 1
		}
	}`

	var data interface{}
	json.Unmarshal([]byte(jsonStr), &data)

	encoder := NewTOONEncoder()
	result := encoder.Encode(data)

	expected := "metadata:\n  page: 1\n  total: 2\nusers[2]{active,id,name}:\n    true,1,Alice\n    false,2,Bob"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}
