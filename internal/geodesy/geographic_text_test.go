package geodesy

import (
	"math"
	"testing"
)

func TestParseLatitudeAcceptsDMSHemisphereSignedAndDecimalDegrees(t *testing.T) {
	want := -(31 + 57.0/60)
	for _, input := range []string{"-31.5700", "31.5700 S", "S 31.5700", "-31.95d", "31.95d S"} {
		got, err := ParseLatitude(input)
		if err != nil {
			t.Fatalf("ParseLatitude(%q): %v", input, err)
		}
		if math.Abs(got-want) > 1e-12 {
			t.Fatalf("ParseLatitude(%q)=%f want %f", input, got, want)
		}
	}
}

func TestParseLongitudeAcceptsEastDMSAndDecimalDegrees(t *testing.T) {
	want := 115 + 51.0/60 + 36.2/3600
	for _, input := range []string{"115.513620 E", "E 115.513620", "115.8600555556d E"} {
		got, err := ParseLongitude(input)
		if err != nil {
			t.Fatalf("ParseLongitude(%q): %v", input, err)
		}
		if math.Abs(got-want) > 1e-9 {
			t.Fatalf("ParseLongitude(%q)=%f want %f", input, got, want)
		}
	}
}

func TestParseGeographicRejectsConflictingHemisphereAndImplicitDecimal(t *testing.T) {
	if _, err := ParseLatitude("-31.95d N"); err == nil {
		t.Fatal("expected conflicting latitude hemisphere rejection")
	}
	if _, err := ParseLatitude("+31.5700 S"); err == nil {
		t.Fatal("expected positive south latitude rejection")
	}
	if _, err := ParseLatitude("-31.95"); err == nil {
		t.Fatal("decimal degrees without d suffix must not be accepted as decimal input")
	}
}

func TestFormatGeographicUsesHemisphereDMS(t *testing.T) {
	if got := FormatLatitude(-31.95); got != "31°57′00.00000″ S" {
		t.Fatalf("latitude=%q", got)
	}
	if got := FormatLongitude(115.86); got != "115°51′36.00000″ E" {
		t.Fatalf("longitude=%q", got)
	}
	if got := FormatLatitude(-(31 + 57.0/60 + 12.3965/3600)); got != "31°57′12.39650″ S" {
		t.Fatalf("fractional seconds=%q", got)
	}
}
