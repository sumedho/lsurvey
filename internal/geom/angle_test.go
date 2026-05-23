package geom

import (
	"math"
	"testing"
)

func TestParseCompactDMS(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"123.3045", 123 + 30.0/60 + 45.0/3600},
		{"123.304567", 123 + 30.0/60 + 45.67/3600},
		{"123.30456789", 123 + 30.0/60 + 45.6789/3600},
		{"12", 12},
	}
	for _, tt := range tests {
		got, err := ParseCompactDMS(tt.in)
		if err != nil {
			t.Fatalf("ParseCompactDMS(%q): %v", tt.in, err)
		}
		if math.Abs(got.Degrees()-tt.want) > 1e-10 {
			t.Fatalf("ParseCompactDMS(%q)=%0.12f want %0.12f", tt.in, got.Degrees(), tt.want)
		}
	}
}

func TestParseCompactDMSRejectsInvalidMinutesSeconds(t *testing.T) {
	for _, in := range []string{"10.6045", "10.3060", "bad"} {
		if _, err := ParseCompactDMS(in); err == nil {
			t.Fatalf("ParseCompactDMS(%q) succeeded", in)
		}
	}
}

func TestParseQuadrantBearing(t *testing.T) {
	tests := []struct {
		tokens []string
		want   float64
	}{
		{[]string{"N", "45.0000", "E"}, 45},
		{[]string{"S", "45.0000", "E"}, 135},
		{[]string{"S", "45.0000", "W"}, 225},
		{[]string{"N", "45.0000", "W"}, 315},
	}
	for _, tt := range tests {
		got, used, err := ParseQuadrantBearing(tt.tokens)
		if err != nil {
			t.Fatal(err)
		}
		if used != 3 {
			t.Fatalf("used=%d want 3", used)
		}
		if math.Abs(got.Degrees()-tt.want) > 1e-10 {
			t.Fatalf("got=%f want=%f", got.Degrees(), tt.want)
		}
	}
}

func TestFormatCompactDMS(t *testing.T) {
	got := AngleFromDegrees(123 + 30.0/60 + 45.67/3600).FormatCompactDMS(2)
	if got != "123.304567" {
		t.Fatalf("got %q want 123.304567", got)
	}
}

func TestFormatDMSWithSymbolsAndHundredths(t *testing.T) {
	got := AngleFromDegrees(123 + 30.0/60 + 45.67/3600).FormatDMS(2)
	if got != "123°30′45.67″" {
		t.Fatalf("got %q want 123°30′45.67″", got)
	}
}
