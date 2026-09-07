package geom

import (
	"math"
	"testing"
)

// Independent arithmetic: legs (30,40), (-90,0), (0,-40) have lengths
// 50,90,40, total 180. Target (-59,2) requires closure corrections (+1,+2).
// Compass cumulative weights: 5/18,14/18,1. Transit east: 1/4,1,1;
// north: 1/2,1/2,1. Translation to MGA coordinates must not affect results.
func TestTraverseAdjustmentReference(t *testing.T) {
	for _, origin := range []Point{{}, {Easting: 500000, Northing: 6500000}} {
		z := 10.0
		points := []Point{
			{ID: "S", Easting: origin.Easting, Northing: origin.Northing},
			{ID: "A", Easting: origin.Easting + 30, Northing: origin.Northing + 40, Elevation: &z, Code: "PEG", Description: "retain"},
			{ID: "B", Easting: origin.Easting - 60, Northing: origin.Northing + 40},
			{ID: "C", Easting: origin.Easting - 60, Northing: origin.Northing},
		}
		target := Point{ID: "T", Easting: origin.Easting - 59, Northing: origin.Northing + 2}
		for _, tc := range []struct {
			method      string
			east, north float64
		}{
			{"compass", 30 + 5.0/18, 40 + 10.0/18}, {"transit", 30.25, 41},
		} {
			r, err := AdjustTraverse(points, target, tc.method)
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(r.Points[0].Easting-origin.Easting-tc.east) > 1e-8 || math.Abs(r.Points[0].Northing-origin.Northing-tc.north) > 1e-8 {
				t.Fatalf("%s: %+v", tc.method, r)
			}
			if r.Points[2].Easting != target.Easting || r.Points[2].Northing != target.Northing {
				t.Fatal("endpoint does not close")
			}
			if r.Length != 180 || math.Abs(r.Misclosure-math.Sqrt(5)) > 1e-12 {
				t.Fatalf("QA: %+v", r)
			}
			if r.Points[0].Code != "PEG" || r.Points[0].Description != "retain" || *r.Points[0].Elevation != 10 {
				t.Fatal("metadata changed")
			}
			if points[1].Easting != origin.Easting+30 {
				t.Fatal("input mutated")
			}
		}
	}
}

func TestTraverseAdjustmentRejectsDegenerateInputs(t *testing.T) {
	for _, tc := range []struct {
		points []Point
		target Point
		method string
	}{
		{[]Point{{}}, Point{}, "compass"},
		{[]Point{{}, {Easting: 0}}, Point{}, "compass"},
		{[]Point{{}, {Easting: 10}}, Point{Easting: 11, Northing: 1}, "transit"},
		{[]Point{{}, {Easting: math.Inf(1)}}, Point{}, "compass"},
		{[]Point{{}, {Easting: 10}}, Point{}, "unknown"},
	} {
		if _, err := AdjustTraverse(tc.points, tc.target, tc.method); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	if _, err := AdjustTraverse([]Point{{}, {Easting: 10}}, Point{Easting: 11}, "transit"); err != nil {
		t.Fatal(err)
	}
}
