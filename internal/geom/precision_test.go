package geom

import (
	"math"
	"testing"
)

func TestSmallGeometryAtMGACoordinates(t *testing.T) {
	e, n := 500000.0, 6500000.0
	p := []Point{{Easting: e, Northing: n}, {Easting: e + .01, Northing: n}, {Easting: e + .01, Northing: n + .02}, {Easting: e, Northing: n + .02}}
	r, ok := Close(p)
	if !ok || math.Abs(r.Area-.0002) > 1e-10 || PolygonZeroArea(p) {
		t.Fatalf("area lost precision: %+v", r)
	}
	intersection, ok := LineIntersection(p[0], p[2], p[1], p[3], "X", "")
	if !ok || math.Abs(intersection.Easting-e-.005) > 1e-8 || math.Abs(intersection.Northing-n-.01) > 1e-8 {
		t.Fatalf("intersection lost precision: %+v", intersection)
	}
}
