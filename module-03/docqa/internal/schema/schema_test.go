package schema

import (
	"encoding/json"
	"testing"
)

type difficultyFixture struct {
	Level  string `json:"level" description:"难度" enum:"low,medium,high"`
	Reason string `json:"reason" description:"一句话原因"`
	Note   string `json:"note,omitempty"`
}

func TestGenerate(t *testing.T) {
	got, err := Generate(difficultyFixture{})
	if err != nil {
		t.Fatal(err)
	}
	var doc Document
	if err := json.Unmarshal([]byte(got), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Type != "object" || doc.AdditionalProperties {
		t.Fatalf("doc = %+v", doc)
	}
	if doc.Properties["level"].Type != "string" {
		t.Fatalf("level = %+v", doc.Properties["level"])
	}
	if len(doc.Properties["level"].Enum) != 3 {
		t.Fatalf("enum = %+v", doc.Properties["level"].Enum)
	}
	if len(doc.Required) != 2 || doc.Required[0] != "level" || doc.Required[1] != "reason" {
		t.Fatalf("required = %+v", doc.Required)
	}
}

func TestGenerateRejectsNonStruct(t *testing.T) {
	if _, err := Generate("x"); err == nil {
		t.Fatal("期望非 struct 错误")
	}
}
