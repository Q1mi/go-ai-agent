package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Document 表示一个最小 JSON Schema 文档。
type Document struct {
	Type                 string              `json:"type"`
	Properties           map[string]Property `json:"properties"`
	Required             []string            `json:"required,omitempty"`
	AdditionalProperties bool                `json:"additionalProperties"`
}

// Property 表示 JSON Schema 中的一个字段定义。
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Items       *Item    `json:"items,omitempty"`
}

// Item 表示数组字段的元素 Schema。
type Item struct {
	Type string `json:"type"`
}

// Generate 根据 Go struct 生成用于结构化输出的 JSON Schema。
//
// 课程示例只覆盖常见基础类型、数组、description tag 和 enum tag，便于学员
// 把结构化输出和普通 Go 类型关联起来。
func Generate(value any) (string, error) {
	if value == nil {
		return "", fmt.Errorf("schema.Generate: value 不能为空")
	}
	typ := reflect.TypeOf(value)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return "", fmt.Errorf("schema.Generate: 只支持 struct，收到 %s", typ.Kind())
	}

	doc := Document{
		Type:                 "object",
		Properties:           make(map[string]Property),
		AdditionalProperties: false,
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, required := jsonName(field)
		if name == "" || name == "-" {
			continue
		}
		property, err := propertyForField(field)
		if err != nil {
			return "", fmt.Errorf("%s: %w", field.Name, err)
		}
		doc.Properties[name] = property
		if required {
			doc.Required = append(doc.Required, name)
		}
	}

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 schema: %w", err)
	}
	return string(raw), nil
}

// jsonName 根据 json tag 计算 Schema 字段名和是否必填。
func jsonName(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("json")
	if tag == "" {
		return lowerCamel(field.Name), true
	}
	parts := strings.Split(tag, ",")
	required := true
	for _, option := range parts[1:] {
		if option == "omitempty" {
			required = false
		}
	}
	return parts[0], required
}

// propertyForField 把一个 struct 字段转换为 JSON Schema property。
func propertyForField(field reflect.StructField) (Property, error) {
	typ := field.Type
	property := Property{
		Type:        schemaType(typ),
		Description: field.Tag.Get("description"),
	}
	if property.Type == "" {
		return Property{}, fmt.Errorf("不支持类型 %s", typ)
	}
	if enum := field.Tag.Get("enum"); enum != "" {
		for _, value := range strings.Split(enum, ",") {
			value = strings.TrimSpace(value)
			if value != "" {
				property.Enum = append(property.Enum, value)
			}
		}
	}
	if typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		itemType := schemaType(typ.Elem())
		if itemType == "" {
			return Property{}, fmt.Errorf("不支持数组元素类型 %s", typ.Elem())
		}
		property.Items = &Item{Type: itemType}
	}
	return property, nil
}

// schemaType 把 Go 类型映射为 JSON Schema 基础类型。
func schemaType(typ reflect.Type) string {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return ""
	}
}

// lowerCamel 把导出的 Go 字段名转换为默认 JSON 字段名。
func lowerCamel(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToLower(name[:1]) + name[1:]
}
