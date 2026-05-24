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

func TestCloseComputesAreaPerimeterAndMisclose(t *testing.T) {
	got, ok := Close([]Point{
		{ID: "1", Easting: 0, Northing: 0},
		{ID: "2", Easting: 4, Northing: 0},
		{ID: "3", Easting: 4, Northing: 3},
	})
	if !ok {
		t.Fatal("expected close result")
	}
	assertClose(t, got.Area, 6)
	assertClose(t, got.Perimeter, 12)
	assertClose(t, got.Misclose.HorizontalDistance, 5)
	assertClose(t, got.Misclose.Azimuth.Degrees(), 233.13010235415598)
}

func TestCloseRejectsTooFewPoints(t *testing.T) {
	if _, ok := Close([]Point{{}, {}}); ok {
		t.Fatal("expected too few points rejection")
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

func TestShiftPoint(t *testing.T) {
	z := 5.0
	got := ShiftPoint(Point{Easting: 10, Northing: 20, Elevation: &z}, 2, -3, &z)
	assertClose(t, got.Easting, 12)
	assertClose(t, got.Northing, 17)
	if got.Elevation == nil {
		t.Fatal("expected elevation")
	}
	assertClose(t, *got.Elevation, 10)
}

func TestRotatePoint(t *testing.T) {
	got := RotatePoint(Point{Easting: 10, Northing: 0}, Point{Easting: 0, Northing: 0}, AngleFromDegrees(90))
	assertClose(t, got.Easting, 0)
	assertClose(t, got.Northing, -10)
}

func TestFitSimilarityTransform(t *testing.T) {
	z1, z2, tz1, tz2 := 1.0, 2.0, 4.0, 5.0
	got, ok := FitSimilarityTransform([]PointPair{
		{Source: Point{Easting: 0, Northing: 0, Elevation: &z1}, Target: Point{Easting: 100, Northing: 200, Elevation: &tz1}},
		{Source: Point{Easting: 10, Northing: 0, Elevation: &z2}, Target: Point{Easting: 100, Northing: 180, Elevation: &tz2}},
		{Source: Point{Easting: 0, Northing: 10}, Target: Point{Easting: 120, Northing: 200}},
	})
	if !ok {
		t.Fatal("expected transform fit")
	}
	assertClose(t, got.DX, 100)
	assertClose(t, got.DY, 200)
	assertClose(t, got.Scale, 2)
	assertClose(t, got.RotationDegrees, 90)
	assertClose(t, got.HorizontalRMS, 0)
	assertClose(t, *got.DZ, 3)
	assertClose(t, *got.VerticalRMS, 0)
	if got.PairCount != 3 || got.VerticalPairs != 2 {
		t.Fatalf("counts=%d/%d want 3/2", got.PairCount, got.VerticalPairs)
	}
}

func TestFitSimilarityTransformReportsNegativeRotationAndResiduals(t *testing.T) {
	got, ok := FitSimilarityTransform([]PointPair{
		{Source: Point{Easting: 0, Northing: 0}, Target: Point{Easting: 5, Northing: 7}},
		{Source: Point{Easting: 10, Northing: 0}, Target: Point{Easting: 5, Northing: 17}},
		{Source: Point{Easting: 0, Northing: 10}, Target: Point{Easting: -4.9, Northing: 7}},
	})
	if !ok {
		t.Fatal("expected transform fit")
	}
	if got.RotationDegrees >= 0 {
		t.Fatalf("angle=%f want negative", got.RotationDegrees)
	}
	if got.HorizontalRMS <= 0 {
		t.Fatalf("hrms=%f want positive residual", got.HorizontalRMS)
	}
	if got.DZ != nil || got.VerticalRMS != nil {
		t.Fatal("2D fit should not report vertical values")
	}
}

func TestFitSimilarityTransformRejectsDegenerateSourcePoints(t *testing.T) {
	_, ok := FitSimilarityTransform([]PointPair{
		{Source: Point{Easting: 1, Northing: 2}, Target: Point{Easting: 10, Northing: 20}},
		{Source: Point{Easting: 1, Northing: 2}, Target: Point{Easting: 11, Northing: 21}},
	})
	if ok {
		t.Fatal("expected degenerate source geometry rejection")
	}
}
