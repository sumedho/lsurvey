package geom

import (
	"math"
	"testing"
)

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-8 {
		t.Fatalf("got %f want %f", got, want)
	}
}

func TestInverse2D(t *testing.T) {
	got := Inverse(Point{Northing: 100, Easting: 100}, Point{Northing: 200, Easting: 200})
	assertClose(t, got.HorizontalDistance, math.Sqrt(20000))
	assertClose(t, got.Azimuth.Degrees(), 45)
	if got.SlopeDistance != nil {
		t.Fatal("2D inverse should not produce slope distance")
	}
}

func TestInverse3D(t *testing.T) {
	z1, z2 := 10.0, 20.0
	got := Inverse(Point{Northing: 0, Easting: 0, Elevation: &z1}, Point{Northing: 30, Easting: 40, Elevation: &z2})
	assertClose(t, got.HorizontalDistance, 50)
	assertClose(t, *got.SlopeDistance, math.Sqrt(2600))
	assertClose(t, *got.DeltaElevation, 10)
	assertClose(t, *got.GradePercent, 20)
}

func TestAngleBetween(t *testing.T) {
	got, ok := AngleBetween(
		Point{Northing: 10, Easting: 0},
		Point{Northing: 0, Easting: 0},
		Point{Northing: 0, Easting: 10},
	)
	if !ok {
		t.Fatal("expected angle")
	}
	assertClose(t, got.Inside.Degrees(), 90)
	assertClose(t, got.Outside.Degrees(), 270)
}

func TestAngleBetweenRejectsZeroLengthLeg(t *testing.T) {
	_, ok := AngleBetween(Point{Northing: 0, Easting: 0}, Point{Northing: 0, Easting: 0}, Point{Northing: 0, Easting: 10})
	if ok {
		t.Fatal("expected zero length angle rejection")
	}
}

func TestRadiateStoresCode(t *testing.T) {
	from := Point{Northing: 0, Easting: 0}
	got := Radiate(from, AngleFromDegrees(90), 10, nil, "2", "PEG")
	assertClose(t, got.Northing, 0)
	assertClose(t, got.Easting, 10)
	if got.Code != "PEG" {
		t.Fatalf("code=%q want PEG", got.Code)
	}
}

func TestRadiate3DUsesSlopeDistanceAndZenith(t *testing.T) {
	z := 10.0
	got := Radiate3D(
		Point{Easting: 100, Northing: 200, Elevation: &z},
		AngleFromDegrees(0),
		10,
		AngleFromDegrees(60),
		"2",
		"CALC",
	)
	assertClose(t, got.Easting, 100)
	assertClose(t, got.Northing, 200+10*math.Sin(60*DegToRad))
	if got.Elevation == nil {
		t.Fatal("expected elevation")
	}
	assertClose(t, *got.Elevation, 15)
	if got.Code != "CALC" {
		t.Fatalf("code=%q want CALC", got.Code)
	}
}

func TestLineIntersection(t *testing.T) {
	got, ok := LineIntersection(
		Point{Northing: 0, Easting: 0},
		Point{Northing: 10, Easting: 10},
		Point{Northing: 10, Easting: 0},
		Point{Northing: 0, Easting: 10},
		"X",
		"IP",
	)
	if !ok {
		t.Fatal("expected intersection")
	}
	assertClose(t, got.Northing, 5)
	assertClose(t, got.Easting, 5)
	if got.Code != "IP" {
		t.Fatalf("code=%q want IP", got.Code)
	}
}

func TestBearingDistanceIntersection(t *testing.T) {
	got, ok := BearingDistanceIntersection(
		Point{Northing: 0, Easting: 0},
		AngleFromDegrees(90),
		Point{Northing: 0, Easting: 5},
		5,
		"near",
		"X",
		"BD",
	)
	if !ok {
		t.Fatal("expected intersection")
	}
	assertClose(t, got.Northing, 0)
	assertClose(t, got.Easting, 0)
}

func TestResectionByBearings(t *testing.T) {
	got, ok := ResectionByBearings(
		[3]Point{
			{ID: "N", Northing: 10, Easting: 0},
			{ID: "E", Northing: 0, Easting: 10},
			{ID: "W", Northing: 0, Easting: -10},
		},
		[3]Angle{
			AngleFromDegrees(0),
			AngleFromDegrees(90),
			AngleFromDegrees(270),
		},
		"X",
		"RS",
	)
	if !ok {
		t.Fatal("expected resection")
	}
	assertClose(t, got.Northing, 0)
	assertClose(t, got.Easting, 0)
	if got.ID != "X" || got.Code != "RS" {
		t.Fatalf("point=%+v", got)
	}
}

func TestResectionByBearingsRejectsDegenerateGeometry(t *testing.T) {
	_, ok := ResectionByBearings(
		[3]Point{
			{Northing: 10, Easting: 0},
			{Northing: 20, Easting: 0},
			{Northing: 30, Easting: 0},
		},
		[3]Angle{
			AngleFromDegrees(0),
			AngleFromDegrees(0),
			AngleFromDegrees(0),
		},
		"X",
		"",
	)
	if ok {
		t.Fatal("expected degenerate resection rejection")
	}
}
