package validate

import (
	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func Feature(f project.Feature, point func(string) (geom.Point, bool), groupExists func(string) bool) error {
	return project.ValidateFeature(f, point, groupExists)
}
func PolygonInvalid(p []geom.Point) bool        { return geom.PolygonInvalid(p) }
func PolygonZeroArea(p []geom.Point) bool       { return geom.PolygonZeroArea(p) }
func PolygonSelfIntersects(p []geom.Point) bool { return geom.PolygonSelfIntersects(p) }
func SegmentsCross(a, b, c, d geom.Point) bool  { return geom.SegmentsCross(a, b, c, d) }
func ValidTerrainRole(role string) bool {
	return role == "none" || role == "standard" || role == "ridge" || role == "drain"
}
