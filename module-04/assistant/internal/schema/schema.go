package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// Schema 表示一个最小 JSON Schema 对象。
type Schema struct {
	Type                 string              `json:"type"`
	Properties           map[string]Property `json:"properties,omitempty"`
	Required             []string            `json:"required,omitempty"`
	AdditionalProperties bool                `json:"additionalProperties"`
}

// Property 表示 JSON Schema 中的字段定义。
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Items       *Item    `json:"items,omitempty"`
}

// Item 表示数组字段的元素类型。
type Item struct {
	Type string `json:"type"`
}

// Generate 根据 Go struct 生成工具参数 Schema。
//
// 练习中的 calculator 和 now 工具都通过它把参数结构暴露给模型。
func Generate(value any) (*Schema, error) {
	if value == nil {
		return nil, fmt.Errorf("schema.Generate: value 不能为空")
	}
	typ := reflect.TypeOf(value)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("schema.Generate: 只支持 struct，收到 %s", typ.Kind())
	}

	out := &Schema{
		Type:                 "object",
		Properties:           make(map[string]Property),
		AdditionalProperties: false,
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name, required := fieldJSONName(field)
		if name == "" || name == "-" {
			continue
		}
		property, err := propertyForField(field)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field.Name, err)
		}
		out.Properties[name] = property
		if required {
			out.Required = append(out.Required, name)
		}
	}
	return out, nil
}

// MustJSON 把 Schema 序列化为模型请求中的 RawMessage。
func MustJSON(s *Schema) json.RawMessage {
	raw, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return raw
}

// fieldJSONName 根据 json tag 计算字段名和是否必填。
func fieldJSONName(field reflect.StructField) (string, bool) {
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

// propertyForField 把一个 struct 字段转换为 Schema 字段定义。
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

// schemaType 把 Go 类型映射到 JSON Schema 基础类型。
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
