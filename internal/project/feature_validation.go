package project

import (
	"fmt"

	"lsurvey/internal/geom"
)

func ValidateFeature(feature Feature, point func(string) (geom.Point, bool), groupExists func(string) bool) error {
	if feature.Kind != FeatureLine && feature.Kind != FeaturePolyline && feature.Kind != FeaturePolygon {
		return fmt.Errorf("unknown feature kind %q", feature.Kind)
	}
	if feature.Kind == FeaturePolygon && feature.TerrainRole != "" {
		return fmt.Errorf("polygons do not support terrain roles")
	}
	if feature.TerrainRole != "" && feature.TerrainRole != "standard" && feature.TerrainRole != "ridge" && feature.TerrainRole != "drain" {
		return fmt.Errorf("invalid terrain role %q", feature.TerrainRole)
	}
	min := 2
	if feature.Kind == FeaturePolygon {
		min = 3
	}
	if len(feature.PointIDs) < min || feature.Kind == FeatureLine && len(feature.PointIDs) != 2 {
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
		if !geom.FinitePoint(pt) {
			return fmt.Errorf("point %q has non-finite coordinates", id)
		}
	}
	if feature.Kind == FeaturePolygon {
		if feature.PointIDs[0] == feature.PointIDs[len(feature.PointIDs)-1] {
			return fmt.Errorf("polygon closure is implicit; do not repeat the first point")
		}
		if geom.PolygonInvalid(points) {
			return fmt.Errorf("polygon %q is zero-area or self-intersecting", feature.ID)
		}
	}
	if feature.GroupID != "" && !groupExists(feature.GroupID) {
		return fmt.Errorf("group %q not found", feature.GroupID)
	}
	return nil
}
