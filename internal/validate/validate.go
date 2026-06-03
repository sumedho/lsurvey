package validate

import (
	"fmt"
	"math"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

const eps = 1e-9

func Feature(feature project.Feature, point func(string) (geom.Point, bool), groupExists func(string) bool) error {
	min := 2
	if feature.Kind == project.FeaturePolygon {
		min = 3
	}
	if len(feature.PointIDs) < min || feature.Kind == project.FeatureLine && len(feature.PointIDs) != 2 {
		return fmt.Errorf("%s %q has invalid point count", feature.Kind, feature.ID)
	}
	points := make([]geom.Point, 0, len(feature.PointIDs))
	for i, id := range feature.PointIDs {
		pt, ok := point(id)
		if !ok {
			return fmt.Errorf("point %q not found", id)
		}
		if i > 0 && id == feature.PointIDs[i-1] {
			return fmt.Errorf("%s %q has duplicate consecutive points", feature.Kind, feature.ID)
		}
		points = append(points, pt)
	}
	if feature.Kind == project.FeaturePolygon {
		if feature.PointIDs[0] == feature.PointIDs[len(feature.PointIDs)-1] {
			return fmt.Errorf("polygon closure is implicit; do not repeat the first point")
		}
		if PolygonInvalid(points) {
			return fmt.Errorf("polygon %q is zero-area or self-intersecting", feature.ID)
		}
	}
	if feature.GroupID != "" && !groupExists(feature.GroupID) {
		return fmt.Errorf("group %q not found", feature.GroupID)
	}
	return nil
}

func PolygonInvalid(points []geom.Point) bool {
	return PolygonZeroArea(points) || PolygonSelfIntersects(points)
}

func PolygonZeroArea(points []geom.Point) bool {
	return math.Abs(polygonTwiceArea(points)) < eps
}

func PolygonSelfIntersects(points []geom.Point) bool {
	for i := range points {
		a, b := points[i], points[(i+1)%len(points)]
		for j := i + 1; j < len(points); j++ {
			if polygonSegmentsAdjacent(i, j, len(points)) {
				continue
			}
			c, d := points[j], points[(j+1)%len(points)]
			if SegmentsCross(a, b, c, d) {
				return true
			}
		}
	}
	return false
}

func polygonSegmentsAdjacent(i, j, n int) bool {
	return i == j || (i+1)%n == j || (j+1)%n == i
}

func SegmentsCross(a, b, c, d geom.Point) bool {
	orient := func(p, q, r geom.Point) float64 {
		return (q.Easting-p.Easting)*(r.Northing-p.Northing) - (q.Northing-p.Northing)*(r.Easting-p.Easting)
	}
	onSegment := func(p, q, r geom.Point) bool {
		return q.Easting >= math.Min(p.Easting, r.Easting)-eps && q.Easting <= math.Max(p.Easting, r.Easting)+eps &&
			q.Northing >= math.Min(p.Northing, r.Northing)-eps && q.Northing <= math.Max(p.Northing, r.Northing)+eps
	}
	o1, o2, o3, o4 := orient(a, b, c), orient(a, b, d), orient(c, d, a), orient(c, d, b)
	if math.Abs(o1) <= eps && onSegment(a, c, b) || math.Abs(o2) <= eps && onSegment(a, d, b) ||
		math.Abs(o3) <= eps && onSegment(c, a, d) || math.Abs(o4) <= eps && onSegment(c, b, d) {
		return true
	}
	return (o1 > 0) != (o2 > 0) && (o3 > 0) != (o4 > 0)
}

func ValidTerrainRole(role string) bool {
	return role == "none" || role == "standard" || role == "ridge" || role == "drain"
}

func polygonTwiceArea(points []geom.Point) float64 {
	area := 0.0
	for i := range points {
		a, b := points[i], points[(i+1)%len(points)]
		area += a.Easting*b.Northing - b.Easting*a.Northing
	}
	return area
}
