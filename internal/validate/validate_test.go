package validate

import (
	"testing"

	"lsurvey/internal/geom"
)

func TestPolygonInvalidRejectsZeroArea(t *testing.T) {
	points := []geom.Point{{Easting: 0, Northing: 0}, {Easting: 1, Northing: 1}, {Easting: 2, Northing: 2}}
	if !PolygonInvalid(points) || !PolygonZeroArea(points) {
		t.Fatal("expected zero-area polygon to be invalid")
	}
}

func TestPolygonInvalidRejectsSelfIntersection(t *testing.T) {
	points := []geom.Point{{Easting: 0, Northing: 0}, {Easting: 10, Northing: 10}, {Easting: 0, Northing: 10}, {Easting: 10, Northing: 0}}
	if !PolygonInvalid(points) || !PolygonSelfIntersects(points) {
		t.Fatal("expected self-intersecting polygon to be invalid")
	}
}

func TestPolygonInvalidAcceptsSimplePolygon(t *testing.T) {
	points := []geom.Point{{Easting: 0, Northing: 0}, {Easting: 10, Northing: 0}, {Easting: 10, Northing: 10}, {Easting: 0, Northing: 10}}
	if PolygonInvalid(points) {
		t.Fatal("expected simple polygon to be valid")
	}
}

func TestPolygonSelfIntersectsSkipsClosingEdgeAdjacency(t *testing.T) {
	points := []geom.Point{{Easting: 0, Northing: 0}, {Easting: 10, Northing: 0}, {Easting: 10, Northing: 10}, {Easting: 0, Northing: 10}}
	if PolygonSelfIntersects(points) {
		t.Fatal("expected closing edge adjacency to be valid")
	}
}

func TestPolygonSelfIntersectsDetectsClosingEdgeCrossing(t *testing.T) {
	points := []geom.Point{{Easting: 0, Northing: 0}, {Easting: 10, Northing: 0}, {Easting: 0, Northing: 10}, {Easting: 10, Northing: 10}}
	if !PolygonSelfIntersects(points) {
		t.Fatal("expected closing edge crossing to be invalid")
	}
}

func TestSegmentsCrossIncludesCollinearOverlap(t *testing.T) {
	a := geom.Point{Easting: 0, Northing: 0}
	b := geom.Point{Easting: 10, Northing: 0}
	c := geom.Point{Easting: 5, Northing: 0}
	d := geom.Point{Easting: 15, Northing: 0}
	if !SegmentsCross(a, b, c, d) {
		t.Fatal("expected overlapping collinear segments to cross")
	}
}

func TestSegmentsCrossRejectsDisjointSegments(t *testing.T) {
	a := geom.Point{Easting: 0, Northing: 0}
	b := geom.Point{Easting: 1, Northing: 0}
	c := geom.Point{Easting: 0, Northing: 1}
	d := geom.Point{Easting: 1, Northing: 1}
	if SegmentsCross(a, b, c, d) {
		t.Fatal("expected disjoint segments not to cross")
	}
}

func TestValidTerrainRole(t *testing.T) {
	for _, role := range []string{"none", "standard", "ridge", "drain"} {
		if !ValidTerrainRole(role) {
			t.Fatalf("expected %q to be valid", role)
		}
	}
	if ValidTerrainRole("wall") {
		t.Fatal("expected wall to be invalid")
	}
}
