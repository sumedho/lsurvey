package project

import (
	"encoding/json"

	"lsurvey/internal/geom"
)

// Clone returns a deep copy of the project suitable for undo/redo snapshots.
func (p *Project) Clone() *Project {
	if p == nil {
		return nil
	}
	cloned := *p
	cloned.Display = cloneDisplaySettings(p.Display)
	cloned.Units = cloneMap(p.Units, cloneString)
	cloned.Points = cloneMap(p.Points, clonePoint)
	cloned.Groups = cloneMap(p.Groups, cloneGroup)
	cloned.PointCodeStyles = cloneMap(p.PointCodeStyles, cloneString)
	cloned.Features = cloneMap(p.Features, cloneFeature)
	cloned.LegacyLines = cloneMap(p.LegacyLines, cloneLine)
	cloned.ContourSets = cloneMap(p.ContourSets, cloneContourSet)
	cloned.Traverse = cloneTraverseState(p.Traverse)
	cloned.GridGround = cloneGridGroundConversion(p.GridGround)
	cloned.HorizontalCRS = cloneHorizontalCRS(p.HorizontalCRS)
	cloned.History = cloneSlice(p.History, cloneHistoryRecord)
	return &cloned
}

func cloneDisplaySettings(settings DisplaySettings) DisplaySettings {
	settings.Precision = cloneIntPtr(settings.Precision)
	return settings
}

func clonePoint(point geom.Point) geom.Point {
	point.Elevation = cloneFloatPtr(point.Elevation)
	return point
}

func cloneFeature(feature Feature) Feature {
	feature.PointIDs = cloneSlice(feature.PointIDs, cloneString)
	return feature
}

func cloneContourSet(set ContourSet) ContourSet {
	set.SourcePoints = cloneSlice(set.SourcePoints, cloneString)
	set.Breaklines = cloneSlice(set.Breaklines, cloneString)
	set.BreaklineRoles = cloneMap(set.BreaklineRoles, cloneString)
	set.BoundaryLines = cloneSlice(set.BoundaryLines, cloneString)
	set.ExclusionLines = cloneSlice(set.ExclusionLines, cloneString)
	set.Generation = cloneContourGenerationSpec(set.Generation)
	set.Diagnostics = cloneSlice(set.Diagnostics, cloneContourDiagnostic)
	set.RawPolylines = cloneSlice(set.RawPolylines, cloneContourPolyline)
	set.Polylines = cloneSlice(set.Polylines, cloneContourPolyline)
	return set
}

func cloneContourGenerationSpec(spec *ContourGenerationSpec) *ContourGenerationSpec {
	if spec == nil {
		return nil
	}
	cloned := *spec
	cloned.Base = cloneFloatPtr(spec.Base)
	cloned.BreaklineIDs = cloneSlice(spec.BreaklineIDs, cloneString)
	cloned.BoundaryCodes = cloneSlice(spec.BoundaryCodes, cloneString)
	cloned.ExclusionCodes = cloneSlice(spec.ExclusionCodes, cloneString)
	cloned.MaxEdge = cloneFloatPtr(spec.MaxEdge)
	return &cloned
}

func cloneContourDiagnostic(diagnostic ContourDiagnostic) ContourDiagnostic {
	diagnostic.PointIDs = cloneSlice(diagnostic.PointIDs, cloneString)
	diagnostic.EdgeIDs = cloneSlice(diagnostic.EdgeIDs, cloneString)
	return diagnostic
}

func cloneContourPolyline(polyline ContourPolyline) ContourPolyline {
	polyline.Vertices = cloneSlice(polyline.Vertices, cloneContourVertex)
	return polyline
}

func cloneHistoryRecord(record HistoryRecord) HistoryRecord {
	record.Created = cloneSlice(record.Created, cloneString)
	record.Updated = cloneSlice(record.Updated, cloneString)
	record.Extra = cloneRawMessage(record.Extra)
	return record
}

func cloneTraverseState(state *TraverseState) *TraverseState {
	if state == nil {
		return nil
	}
	cloned := *state
	cloned.LegPointIDs = cloneSlice(state.LegPointIDs, cloneString)
	return &cloned
}

func cloneGridGroundConversion(conversion *GridGroundConversion) *GridGroundConversion {
	if conversion == nil {
		return nil
	}
	cloned := *conversion
	return &cloned
}

func cloneHorizontalCRS(crs *HorizontalCRS) *HorizontalCRS {
	if crs == nil {
		return nil
	}
	cloned := *crs
	return &cloned
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	return cloneSlice(raw, cloneByte)
}

func cloneMap[K comparable, V any](values map[K]V, cloneValue func(V) V) map[K]V {
	if values == nil {
		return nil
	}
	cloned := make(map[K]V, len(values))
	for key, value := range values {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneSlice[T any](values []T, cloneValue func(T) T) []T {
	if values == nil {
		return nil
	}
	cloned := make([]T, len(values))
	for i, value := range values {
		cloned[i] = cloneValue(value)
	}
	return cloned
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneFloatPtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneString(value string) string { return value }

func cloneByte(value byte) byte { return value }

func cloneGroup(value Group) Group { return value }

func cloneLine(value Line) Line { return value }

func cloneContourVertex(value ContourVertex) ContourVertex { return value }
