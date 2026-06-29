package schema

import "testing"

type fixture struct {
	Expr string `json:"expr" description:"算术表达式"`
	Mode string `json:"mode,omitempty" enum:"fast,exact"`
}

func TestGenerate(t *testing.T) {
	got, err := Generate(fixture{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "object" {
		t.Fatalf("type = %q", got.Type)
	}
	if got.Properties["expr"].Type != "string" {
		t.Fatalf("expr = %+v", got.Properties["expr"])
	}
	if len(got.Required) != 1 || got.Required[0] != "expr" {
		t.Fatalf("required = %+v", got.Required)
	}
	if len(got.Properties["mode"].Enum) != 2 {
		t.Fatalf("enum = %+v", got.Properties["mode"].Enum)
	}
}
