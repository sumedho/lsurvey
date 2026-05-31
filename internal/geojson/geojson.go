package geojson

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
	"lsurvey/internal/validate"
)

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
}

type feature struct {
	Type       string         `json:"type"`
	ID         any            `json:"id,omitempty"`
	Geometry   geometry       `json:"geometry"`
	Properties map[string]any `json:"properties,omitempty"`
}

type geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

type importState struct {
	points   map[string]geom.Point
	features map[string]project.Feature
	groups   map[string]project.Group
	styles   map[string]string
}

func ImportFile(path string, p *project.Project) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	return Import(f, p)
}

func ExportFile(path string, p *project.Project) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = Export(f, p)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func Import(r io.Reader, p *project.Project) (int, int, error) {
	var fc featureCollection
	if err := json.NewDecoder(r).Decode(&fc); err != nil {
		return 0, 0, err
	}
	if fc.Type != "FeatureCollection" {
		return 0, 0, fmt.Errorf("unsupported GeoJSON type %q", fc.Type)
	}
	state := importState{clonePoints(p.Points), cloneFeatures(p.Features), cloneGroups(p.Groups), cloneMappings(p.PointCodeStyles)}
	points, features := 0, 0
	for _, pass := range []string{"Point", "geometry"} {
		for i, feat := range fc.Features {
			if (pass == "Point") != (feat.Geometry.Type == "Point") {
				continue
			}
			addedPoints, addedFeatures, err := importFeature(&state, feat)
			if err != nil {
				return 0, 0, fmt.Errorf("feature %d: %w", i+1, err)
			}
			points += addedPoints
			features += addedFeatures
		}
	}
	p.Points, p.Features, p.Groups = state.points, state.features, state.groups
	if points > 0 || features > 0 {
		p.MarkContoursStale("imported terrain input changed")
	}
	return points, features, nil
}

func Export(w io.Writer, p *project.Project) error {
	fc := featureCollection{Type: "FeatureCollection", Features: make([]feature, 0, len(p.Points)+len(p.Features))}
	for _, pt := range p.SortedPoints() {
		fc.Features = append(fc.Features, exportPoint(pt, p.Groups[pt.GroupID]))
	}
	for _, item := range p.SortedFeatures() {
		fc.Features = append(fc.Features, exportFeature(item, p))
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(fc)
}

func exportPoint(pt geom.Point, group project.Group) feature {
	props := map[string]any{"feature_type": "point", "id": pt.ID, "code": pt.Code, "description": pt.Description}
	addGroupProperties(props, pt.GroupID, group)
	return feature{Type: "Feature", ID: pt.ID, Geometry: geometry{Type: "Point", Coordinates: mustMarshal(pointCoordinate(pt))}, Properties: props}
}

func exportFeature(item project.Feature, p *project.Project) feature {
	coords := make([][]float64, 0, len(item.PointIDs)+1)
	for _, id := range item.PointIDs {
		coords = append(coords, pointCoordinate(p.Points[id]))
	}
	geometryType := "LineString"
	raw := any(coords)
	if item.Kind == project.FeaturePolygon {
		coords = append(coords, coords[0])
		geometryType = "Polygon"
		raw = [][][]float64{coords}
	}
	props := map[string]any{
		"feature_type": item.Kind, "id": item.ID, "point_ids": strings.Join(item.PointIDs, ","),
		"code": item.Code, "description": item.Description, "terrain_role": item.TerrainRole,
	}
	addGroupProperties(props, item.GroupID, p.Groups[item.GroupID])
	return feature{Type: "Feature", ID: item.ID, Geometry: geometry{Type: geometryType, Coordinates: mustMarshal(raw)}, Properties: props}
}

func addGroupProperties(props map[string]any, id string, group project.Group) {
	if id == "" {
		return
	}
	props["group"] = id
	props["layer"] = group.Layer
	props["color"] = group.Color
}

func importFeature(state *importState, feat feature) (int, int, error) {
	switch feat.Geometry.Type {
	case "Point":
		coord, err := parseCoordinate(feat.Geometry.Coordinates)
		if err != nil {
			return 0, 0, err
		}
		id := first(propertyString(feat.Properties, "id"), scalarString(feat.ID), nextPointID(state.points))
		groupID, err := importGroup(state, feat.Properties)
		if err != nil {
			return 0, 0, err
		}
		pt := makePoint(id, coord)
		pt.Code, pt.Description, pt.GroupID = propertyString(feat.Properties, "code"), propertyString(feat.Properties, "description", "desc"), groupID
		pt = applyCodeStyle(state, pt)
		if existing, ok := state.points[id]; ok && !pointsEqual(existing, pt) {
			return 0, 0, fmt.Errorf("point %q already exists", id)
		}
		if _, ok := state.points[id]; ok {
			return 0, 0, nil
		}
		state.points[id] = pt
		return 1, 0, nil
	case "LineString":
		var coords [][]float64
		if err := json.Unmarshal(feat.Geometry.Coordinates, &coords); err != nil || len(coords) < 2 {
			return 0, 0, fmt.Errorf("LineString requires at least 2 coordinates")
		}
		kind := propertyString(feat.Properties, "feature_type")
		if kind != project.FeatureLine {
			kind = project.FeaturePolyline
		}
		return importGeometry(state, feat, coords, kind)
	case "Polygon":
		var rings [][][]float64
		if err := json.Unmarshal(feat.Geometry.Coordinates, &rings); err != nil || len(rings) != 1 || len(rings[0]) < 4 {
			return 0, 0, fmt.Errorf("Polygon requires one closed exterior ring")
		}
		coords := rings[0]
		if !coordinatesEqual(coords[0], coords[len(coords)-1]) {
			return 0, 0, fmt.Errorf("Polygon ring must be closed")
		}
		return importGeometry(state, feat, coords[:len(coords)-1], project.FeaturePolygon)
	default:
		return 0, 0, fmt.Errorf("unsupported geometry type %q", feat.Geometry.Type)
	}
}

func importGeometry(state *importState, feat feature, coords [][]float64, kind string) (int, int, error) {
	ids := strings.Split(propertyString(feat.Properties, "point_ids"), ",")
	if len(ids) != len(coords) || len(ids) == 1 && ids[0] == "" {
		ids = nil
	}
	added := 0
	pointIDs := make([]string, len(coords))
	for i, coord := range coords {
		if len(coord) < 2 || len(coord) > 3 {
			return 0, 0, fmt.Errorf("coordinate must have 2 or 3 numbers")
		}
		id := ""
		if ids != nil {
			id = ids[i]
		} else {
			id = nextPointID(state.points)
		}
		pt := makePoint(id, coord)
		if existing, ok := state.points[id]; ok {
			if !samePosition(existing, pt) {
				return 0, 0, fmt.Errorf("point %q already exists", id)
			}
		} else {
			state.points[id] = pt
			added++
		}
		pointIDs[i] = id
	}
	id := first(propertyString(feat.Properties, "id"), scalarString(feat.ID), nextFeatureID(state.features))
	if _, exists := state.features[id]; exists {
		return 0, 0, fmt.Errorf("feature %q already exists", id)
	}
	groupID, err := importGroup(state, feat.Properties)
	if err != nil {
		return 0, 0, err
	}
	item := project.Feature{ID: id, Kind: kind, PointIDs: pointIDs, Code: propertyString(feat.Properties, "code"), Description: propertyString(feat.Properties, "description", "desc"), TerrainRole: propertyString(feat.Properties, "terrain_role"), GroupID: groupID}
	if kind == project.FeaturePolygon {
		item.TerrainRole = ""
	}
	if err := validateImportedFeature(state, item); err != nil {
		return 0, 0, err
	}
	state.features[id] = item
	return added, 1, nil
}

func importGroup(state *importState, props map[string]any) (string, error) {
	id := propertyString(props, "group")
	if id == "" {
		return "", nil
	}
	layer := propertyString(props, "layer")
	color, err := strconv.Atoi(propertyString(props, "color"))
	if layer == "" || err != nil || color < 1 || color > 255 {
		return "", fmt.Errorf("group %q requires layer and color 1..255", id)
	}
	group := project.Group{ID: id, Layer: layer, Color: color}
	if old, ok := state.groups[id]; ok && old != group {
		return "", fmt.Errorf("conflicting definition for group %q", id)
	}
	for existingID, old := range state.groups {
		if existingID != id && strings.EqualFold(old.Layer, layer) {
			return "", fmt.Errorf("group layer %q already exists", layer)
		}
	}
	state.groups[id] = group
	return id, nil
}

func validateImportedFeature(state *importState, item project.Feature) error {
	if item.Kind != project.FeaturePolygon && item.TerrainRole != "" && (item.TerrainRole == "none" || !validate.ValidTerrainRole(item.TerrainRole)) {
		return fmt.Errorf("invalid terrain_role %q", item.TerrainRole)
	}
	if item.Kind != project.FeaturePolygon {
		return nil
	}
	points := make([]geom.Point, 0, len(item.PointIDs))
	for _, id := range item.PointIDs {
		points = append(points, state.points[id])
	}
	if validate.PolygonZeroArea(points) {
		return fmt.Errorf("polygon %q is zero-area", item.ID)
	}
	if validate.PolygonSelfIntersects(points) {
		return fmt.Errorf("polygon %q self-intersects", item.ID)
	}
	return nil
}

func pointCoordinate(pt geom.Point) []float64 {
	out := []float64{pt.Easting, pt.Northing}
	if pt.Elevation != nil {
		out = append(out, *pt.Elevation)
	}
	return out
}

func parseCoordinate(raw json.RawMessage) ([]float64, error) {
	var coord []float64
	if err := json.Unmarshal(raw, &coord); err != nil || len(coord) < 2 || len(coord) > 3 {
		return nil, fmt.Errorf("coordinate must have 2 or 3 numbers")
	}
	return coord, nil
}

func makePoint(id string, coord []float64) geom.Point {
	pt := geom.Point{ID: id, Easting: coord[0], Northing: coord[1]}
	if len(coord) == 3 {
		z := coord[2]
		pt.Elevation = &z
	}
	return pt
}

func pointsEqual(a, b geom.Point) bool {
	return samePosition(a, b) && a.Code == b.Code && a.Description == b.Description && a.GroupID == b.GroupID
}

func samePosition(a, b geom.Point) bool {
	if a.Easting != b.Easting || a.Northing != b.Northing || (a.Elevation == nil) != (b.Elevation == nil) {
		return false
	}
	return a.Elevation == nil || *a.Elevation == *b.Elevation
}

func coordinatesEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func propertyString(props map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := props[key]; ok {
			return scalarString(value)
		}
	}
	return ""
}

func scalarString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func mustMarshal(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func clonePoints(src map[string]geom.Point) map[string]geom.Point {
	dst := make(map[string]geom.Point, len(src))
	for id, pt := range src {
		dst[id] = pt
	}
	return dst
}

func cloneFeatures(src map[string]project.Feature) map[string]project.Feature {
	dst := make(map[string]project.Feature, len(src))
	for id, item := range src {
		item.PointIDs = append([]string(nil), item.PointIDs...)
		dst[id] = item
	}
	return dst
}

func cloneGroups(src map[string]project.Group) map[string]project.Group {
	dst := make(map[string]project.Group, len(src))
	for id, group := range src {
		dst[id] = group
	}
	return dst
}

func cloneMappings(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for code, groupID := range src {
		dst[code] = groupID
	}
	return dst
}

func applyCodeStyle(state *importState, pt geom.Point) geom.Point {
	if pt.GroupID == "" {
		if groupID, ok := state.styles[pt.Code]; ok {
			if _, exists := state.groups[groupID]; exists {
				pt.GroupID = groupID
			}
		}
	}
	return pt
}

func nextPointID(points map[string]geom.Point) string {
	max := 0
	for id := range points {
		if n, err := strconv.Atoi(id); err == nil && n > max {
			max = n
		}
	}
	return strconv.Itoa(max + 1)
}

func nextFeatureID(features map[string]project.Feature) string {
	max := 0
	for id := range features {
		if len(id) > 1 && id[0] == 'L' {
			if n, err := strconv.Atoi(id[1:]); err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("L%d", max+1)
}
