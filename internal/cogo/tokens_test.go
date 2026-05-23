package cogo

import "testing"

func TestTokenizeQuotedValues(t *testing.T) {
	got, err := tokenize(`line edit L1 desc="this is a line" code=BDY`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"line", "edit", "L1", "desc=this is a line", "code=BDY"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("field %d=%q want %q", i, got[i], want[i])
		}
	}
}

func TestTokenizeRejectsUnterminatedQuote(t *testing.T) {
	if _, err := tokenize(`pt edit 1 desc="bad`); err == nil {
		t.Fatal("expected quote error")
	}
}
