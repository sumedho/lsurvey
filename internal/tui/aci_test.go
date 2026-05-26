package tui

import "testing"

func TestACIPaletteProvidesNominalColors(t *testing.T) {
	colors := allACIColors()
	if len(colors) != 255 {
		t.Fatalf("colors=%d want 255", len(colors))
	}
	for index, want := range map[int]ACIColor{
		1:   {Index: 1, Name: "Red", R: 255, G: 0, B: 0},
		2:   {Index: 2, Name: "Yellow", R: 255, G: 255, B: 0},
		7:   {Index: 7, Name: "White/Black", R: 255, G: 255, B: 255},
		10:  {Index: 10, R: 255, G: 0, B: 0},
		11:  {Index: 11, R: 255, G: 127, B: 127},
		12:  {Index: 12, R: 165, G: 0, B: 0},
		250: {Index: 250, R: 51, G: 51, B: 51},
		255: {Index: 255, R: 255, G: 255, B: 255},
	} {
		got, ok := aciColor(index)
		if !ok || got != want {
			t.Fatalf("ACI %d=%+v ok=%v want %+v", index, got, ok, want)
		}
	}
	if _, ok := aciColor(0); ok {
		t.Fatal("ACI 0 should be invalid for style groups")
	}
}
