package llm

import "testing"

func TestParseIntoExtractsJSONFromCodeFence(t *testing.T) {
	type payload struct {
		Route string `json:"route"`
	}
	got, err := ParseInto[payload]("```json\n{\"route\":\"code_review\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if got.Route != "code_review" {
		t.Fatalf("Route = %q", got.Route)
	}
}

func TestExtractJSONWithLeadingText(t *testing.T) {
	got, err := ExtractJSON("结果如下：{\"pass\":true,\"score\":90}，请查收")
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"pass":true,"score":90}` {
		t.Fatalf("ExtractJSON() = %q", got)
	}
}
