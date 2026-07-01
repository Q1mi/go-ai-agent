package calc

import "testing"

func TestEval(t *testing.T) {
	value, err := Eval("12*(3+4)")
	if err != nil {
		t.Fatal(err)
	}
	if got := Format(value); got != "84" {
		t.Fatalf("Format(Eval()) = %q", got)
	}
}

func TestEvalRejectsDivideByZero(t *testing.T) {
	if _, err := Eval("1/0"); err == nil {
		t.Fatal("expected divide by zero error")
	}
}
