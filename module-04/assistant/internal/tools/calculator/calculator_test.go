package calculator

import "testing"

func TestEval(t *testing.T) {
	tests := []struct {
		expr string
		want float64
	}{
		{expr: "1+2*3", want: 7},
		{expr: "(1+2)*3", want: 9},
		{expr: "-1 + 2", want: 1},
		{expr: "8/4/2", want: 1},
	}
	for _, tt := range tests {
		got, err := Eval(tt.expr)
		if err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Fatalf("Eval(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestEvalRejectsInvalid(t *testing.T) {
	for _, expr := range []string{"", "1+", "1/0", "abc"} {
		if _, err := Eval(expr); err == nil {
			t.Fatalf("Eval(%q) 期望错误", expr)
		}
	}
}
