package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Schema 是本课程使用的最小 JSON Schema 表达。
type Schema struct {
	Type                 string              `json:"type"`
	Properties           map[string]Property `json:"properties,omitempty"`
	Required             []string            `json:"required,omitempty"`
	AdditionalProperties bool                `json:"additionalProperties"`
}

// Property 表示对象字段。
type Property struct {
	Type        string    `json:"type,omitempty"`
	Description string    `json:"description,omitempty"`
	Items       *Property `json:"items,omitempty"`
}

// Generate 从结构体类型生成 JSON Schema。
func Generate(value any) (*Schema, error) {
	t := reflect.TypeOf(value)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("schema.Generate 只支持 struct")
	}
	out := &Schema{
		Type:                 "object",
		Properties:           map[string]Property{},
		AdditionalProperties: false,
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, required := jsonName(field)
		if name == "" || name == "-" {
			continue
		}
		prop := Property{
			Type:        jsonType(field.Type),
			Description: firstTag(field, "desc", "description"),
		}
		if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
			prop.Type = "array"
			prop.Items = &Property{Type: jsonType(field.Type.Elem())}
		}
		out.Properties[name] = prop
		if required {
			out.Required = append(out.Required, name)
		}
	}
	return out, nil
}

// MustJSON 把 Schema 转成 raw JSON。
func MustJSON(schema *Schema) json.RawMessage {
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return raw
}

func jsonName(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name, true
	}
	parts := strings.Split(tag, ",")
	required := true
	for _, part := range parts[1:] {
		if part == "omitempty" {
			required = false
		}
	}
	return parts[0], required
}

func firstTag(field reflect.StructField, names ...string) string {
	for _, name := range names {
		if value := field.Tag.Get(name); value != "" {
			return value
		}
	}
	return ""
}

func jsonType(t reflect.Type) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Struct:
		return "object"
	default:
		return "string"
	}
}
