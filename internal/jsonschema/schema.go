// Package jsonschema builds the small subset of JSON Schema used by the tool
// definitions, preserving the declaration order of object properties.
package jsonschema

import (
	"bytes"
	"encoding/json"
)

// Property is one entry of an ordered property map.
type Property struct {
	Name   string
	Schema any
}

// Properties keeps insertion order when marshalled, which encoding/json does
// not do for plain maps.
type Properties []Property

// MarshalJSON renders the properties as a JSON object in declaration order.
func (p Properties) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for index, property := range p {
		if index > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(property.Name)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		value, err := json.Marshal(property.Schema)
		if err != nil {
			return nil, err
		}
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// Object builds an object schema with ordered properties and no additional
// properties allowed.
func Object(properties Properties, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

// Prop is a shorthand for one ordered property entry.
func Prop(name string, schema any) Property {
	return Property{Name: name, Schema: schema}
}

// String builds a string schema.
func String(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

// Number builds a number schema.
func Number(description string) map[string]any {
	return map[string]any{"type": "number", "description": description}
}

// Boolean builds a boolean schema.
func Boolean(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

// Enum builds a string schema restricted to the given values.
func Enum(values []string, description string) map[string]any {
	return map[string]any{"type": "string", "enum": values, "description": description}
}
