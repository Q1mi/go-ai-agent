package schema

import (
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	type Metadata struct {
		Source string `json:"source"`
	}
	type GetWeatherArgs struct {
		City      string            `json:"city" desc:"城市名"`
		Days      int               `json:"days,omitempty" desc:"预报天数，默认 1"`
		Aliases   []string          `json:"aliases,omitempty"`
		Labels    map[string]string `json:"labels,omitempty"`
		Requested *time.Time        `json:"requested_at,omitempty"`
		Metadata
		Ignored string `json:"-"`
	}

	got, err := Generate(GetWeatherArgs{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "object" {
		t.Fatalf("Type = %q", got.Type)
	}
	if got.Properties["city"].Description != "城市名" {
		t.Fatalf("city schema = %+v", got.Properties["city"])
	}
	if got.Properties["aliases"].Items.Type != "string" {
		t.Fatalf("aliases schema = %+v", got.Properties["aliases"])
	}
	if got.Properties["labels"].AdditionalProperties.Type != "string" {
		t.Fatalf("labels schema = %+v", got.Properties["labels"])
	}
	if got.Properties["requested_at"].Format != "date-time" {
		t.Fatalf("requested_at schema = %+v", got.Properties["requested_at"])
	}
	if _, ok := got.Properties["source"]; !ok {
		t.Fatal("匿名字段未展开")
	}
	if _, ok := got.Properties["Ignored"]; ok {
		t.Fatal("json:\"-\" 字段不应出现")
	}
	if len(got.Required) != 2 || got.Required[0] != "city" || got.Required[1] != "source" {
		t.Fatalf("Required = %#v", got.Required)
	}
}

func TestGenerateRejectsCycle(t *testing.T) {
	type Node struct {
		Next *Node `json:"next,omitempty"`
	}
	if _, err := Generate(Node{}); err == nil {
		t.Fatal("期望循环类型错误")
	}
}
