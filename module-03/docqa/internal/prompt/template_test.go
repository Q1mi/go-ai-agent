package prompt

import (
	"strings"
	"testing"
)

func TestTemplateRender(t *testing.T) {
	tmpl, err := New("hello", "你好，{{.Name}}")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tmpl.Render(map[string]string{"Name": "Go"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "你好，Go" {
		t.Fatalf("got %q", got)
	}
}

func TestTemplateMissingKeyReturnsError(t *testing.T) {
	tmpl, err := New("missing", "{{.Name}} {{.Title}}")
	if err != nil {
		t.Fatal(err)
	}
	_, err = tmpl.Render(map[string]string{"Name": "Go"})
	if err == nil || !strings.Contains(err.Error(), "map has no entry for key") {
		t.Fatalf("err = %v", err)
	}
}
