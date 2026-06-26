package schema

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Description          string             `json:"description,omitempty"`
	Format               string             `json:"format,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	AdditionalProperties *Schema            `json:"additionalProperties,omitempty"`
	Required             []string           `json:"required,omitempty"`
}

var timeType = reflect.TypeOf(time.Time{})

func Generate(value any) (*Schema, error) {
	if value == nil {
		return nil, fmt.Errorf("不能为 nil 生成 JSON Schema")
	}
	return generateType(reflect.TypeOf(value), make(map[reflect.Type]bool))
}

func generateType(t reflect.Type, visiting map[reflect.Type]bool) (*Schema, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t == timeType {
		return &Schema{Type: "string", Format: "date-time"}, nil
	}

	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}, nil
	case reflect.Bool:
		return &Schema{Type: "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}, nil
	case reflect.Slice, reflect.Array:
		items, err := generateType(t.Elem(), visiting)
		if err != nil {
			return nil, err
		}
		return &Schema{Type: "array", Items: items}, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("JSON object 的 map key 必须是 string，得到 %s", t.Key())
		}
		valueSchema, err := generateType(t.Elem(), visiting)
		if err != nil {
			return nil, err
		}
		return &Schema{Type: "object", AdditionalProperties: valueSchema}, nil
	case reflect.Struct:
		if visiting[t] {
			return nil, fmt.Errorf("检测到循环类型 %s", t)
		}
		visiting[t] = true
		defer delete(visiting, t)
		return generateStruct(t, visiting)
	default:
		return nil, fmt.Errorf("不支持为 %s 生成 JSON Schema", t)
	}
}

func generateStruct(t reflect.Type, visiting map[reflect.Type]bool) (*Schema, error) {
	result := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		name, omitEmpty, skip := parseJSONTag(field)
		if skip {
			continue
		}

		fieldSchema, err := generateType(field.Type, visiting)
		if err != nil {
			return nil, fmt.Errorf("字段 %s: %w", field.Name, err)
		}
		fieldSchema.Description = field.Tag.Get("desc")

		if field.Anonymous && field.Tag.Get("json") == "" && fieldSchema.Type == "object" {
			for propertyName, propertySchema := range fieldSchema.Properties {
				result.Properties[propertyName] = propertySchema
			}
			result.Required = append(result.Required, fieldSchema.Required...)
			continue
		}

		result.Properties[name] = fieldSchema
		if !omitEmpty {
			result.Required = append(result.Required, name)
		}
	}
	return result, nil
}

func parseJSONTag(field reflect.StructField) (name string, omitEmpty bool, skip bool) {
	name = field.Name
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	if tag == "" {
		return name, false, false
	}

	parts := strings.Split(tag, ",")
	if parts[0] != "" {
		name = parts[0]
	}
	for _, option := range parts[1:] {
		if option == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}
