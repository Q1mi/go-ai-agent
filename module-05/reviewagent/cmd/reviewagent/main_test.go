package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestReadInputFromArgs(t *testing.T) {
	input, err := readInput(config{inputParts: []string{"func", "main(){}"}}, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if input != "func main(){}" {
		t.Fatalf("input = %q", input)
	}
}

func TestReadInputFromStdin(t *testing.T) {
	input, err := readInput(config{}, strings.NewReader("package main\n"))
	if err != nil {
		t.Fatal(err)
	}
	if input != "package main" {
		t.Fatalf("input = %q", input)
	}
}

func TestParseFlags(t *testing.T) {
	var stderr bytes.Buffer
	cfg, err := parseFlags([]string{"--format=json", "--max-rounds=3", "--timeout=2s", "hi"}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.format != "json" || cfg.maxRounds != 3 || cfg.timeout != 2*time.Second {
		t.Fatalf("cfg = %+v", cfg)
	}
}
