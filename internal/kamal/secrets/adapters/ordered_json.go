package adapters

// Reads JSON objects in key order, as Ruby's JSON.parse builds a Hash that keeps it.
// encoding/json decodes objects into Go maps, which lose the order.

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// jsonField is one key and its raw value.
type jsonField struct {
	Key   string
	Value json.RawMessage
}

// isJSONObject reports whether text is valid JSON with an object at the top.
func isJSONObject(text []byte) bool {
	return json.Valid(text) && bytes.HasPrefix(bytes.TrimSpace(text), []byte("{"))
}

// objectFields returns the fields of a JSON object in order.
//
//	fields, err := objectFields([]byte(`{"b":1,"a":2}`)) // b, then a
func objectFields(text []byte) ([]jsonField, error) {
	decoder := json.NewDecoder(bytes.NewReader(text))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("want a JSON object, got %.40q", text)
	}
	var fields []jsonField
	for decoder.More() {
		field, err := nextField(decoder)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func nextField(decoder *json.Decoder) (jsonField, error) {
	token, err := decoder.Token()
	if err != nil {
		return jsonField{}, err
	}
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		return jsonField{}, err
	}
	return jsonField{Key: token.(string), Value: value}, nil
}

// stringifyJSON returns a JSON string's text, and any other JSON value in compact
// form, as Kamal's stringify_secret_value does with JSON.dump: 5432 gives "5432", and
// {"read": "r"} gives `{"read":"r"}`.
func stringifyJSON(value json.RawMessage) string {
	// Unmarshal also accepts null into a string, so only a quoted value is a string.
	var text string
	if bytes.HasPrefix(bytes.TrimSpace(value), []byte(`"`)) && json.Unmarshal(value, &text) == nil {
		return text
	}
	var compact bytes.Buffer
	if json.Compact(&compact, value) != nil {
		return string(value)
	}
	return compact.String()
}
