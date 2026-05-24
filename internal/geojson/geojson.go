package geojson

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

const (
	featureTypePoint = "point"
	featureTypeLine  = "line"
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
	points map[string]geom.Point
	lines  map[string]project.Line
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

	state := importState{
		points: clonePoints(p.Points),
		lines:  cloneLines(p.Lines),
	}
	pointCount := 0
	lineCount := 0
	pointFeatures := make([]feature, 0, len(fc.Features))
	otherFeatures := make([]feature, 0, len(fc.Features))
	for _, feat := range fc.Features {
		if feat.Geometry.Type == "Point" {
			pointFeatures = append(pointFeatures, feat)
			continue
		}
		otherFeatures = append(otherFeatures, feat)
	}
	for idx, feat := range pointFeatures {
		addedPoints, addedLines, err := importFeature(state, feat)
		if err != nil {
			return pointCount, lineCount, fmt.Errorf("feature %d: %w", idx+1, err)
		}
		pointCount += addedPoints
		lineCount += addedLines
	}
	for idx, feat := range otherFeatures {
		addedPoints, addedLines, err := importFeature(state, feat)
		if err != nil {
			return pointCount, lineCount, fmt.Errorf("feature %d: %w", idx+1+len(pointFeatures), err)
		}
		pointCount += addedPoints
		lineCount += addedLines
	}

	p.Points = state.points
	p.Lines = state.lines
	if pointCount > 0 || lineCount > 0 {
		p.MarkContoursStale("imported terrain input changed")
	}
	return pointCount, lineCount, nil
}

func Export(w io.Writer, p *project.Project) error {
	fc := featureCollection{
		Type:     "FeatureCollection",
		Features: make([]feature, 0, len(p.Points)+len(p.Lines)),
	}
	for _, pt := range p.SortedPoints() {
		fc.Features = append(fc.Features, exportPointFeature(pt))
	}
	for _, line := range p.SortedLines() {
		from, ok1 := p.Points[line.From]
		to, ok2 := p.Points[line.To]
		if !ok1 || !ok2 {
			continue
		}
		fc.Features = append(fc.Features, exportLineFeature(line, from, to))
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(fc)
}

func exportPointFeature(pt geom.Point) feature {
	coords := []float64{pt.Easting, pt.Northing}
	if pt.Elevation != nil {
		coords = append(coords, *pt.Elevation)
	}
	return feature{
		Type: "Feature",
		ID:   pt.ID,
		Geometry: geometry{
			Type:        "Point",
			Coordinates: mustMarshal(coords),
		},
		Properties: map[string]any{
			"feature_type": featureTypePoint,
			"id":           pt.ID,
			"code":         pt.Code,
			"description":  pt.Description,
		},
	}
}

func exportLineFeature(line project.Line, from, to geom.Point) feature {
	coords := [][]float64{
		{from.Easting, from.Northing},
		{to.Easting, to.Northing},
	}
	if from.Elevation != nil && to.Elevation != nil {
		coords = [][]float64{
			{from.Easting, from.Northing, *from.Elevation},
			{to.Easting, to.Northing, *to.Elevation},
		}
	}
	return feature{
		Type: "Feature",
		ID:   line.ID,
		Geometry: geometry{
			Type:        "LineString",
			Coordinates: mustMarshal(coords),
		},
		Properties: map[string]any{
			"feature_type": featureTypeLine,
			"id":           line.ID,
			"code":         line.Code,
			"description":  line.Description,
			"terrain_role": line.TerrainRole,
			"from":         line.From,
			"to":           line.To,
		},
	}
}

func importFeature(state importState, feat feature) (int, int, error) {
	if feat.Type != "Feature" {
		return 0, 0, fmt.Errorf("unsupported feature type %q", feat.Type)
	}
	switch feat.Geometry.Type {
	case "Point":
		pt, err := importPointFeature(state, feat)
		if err != nil {
			return 0, 0, err
		}
		state.points[pt.ID] = pt
		return 1, 0, nil
	case "LineString":
		lines, points, err := importLineStringFeature(state, feat)
		return points, lines, err
	case "MultiLineString":
		lines, points, err := importMultiLineStringFeature(state, feat)
		return points, lines, err
	default:
		return 0, 0, fmt.Errorf("unsupported geometry type %q", feat.Geometry.Type)
	}
}

func importPointFeature(state importState, feat feature) (geom.Point, error) {
	coord, err := parseCoordinate(feat.Geometry.Coordinates)
	if err != nil {
		return geom.Point{}, err
	}
	id := propertyString(feat.Properties, "id")
	if id == "" {
		id = featureIDString(feat.ID)
	}
	if id == "" {
		id = nextPointID(state.points)
	}
	return ensurePoint(state, id, coord, propertyString(feat.Properties, "code"), propertyString(feat.Properties, "description", "desc"), true)
}

func ensurePoint(state importState, id string, coord []float64, code, description string, compareAttrs bool) (geom.Point, error) {
	pt := geom.Point{
		ID:          id,
		Easting:     coord[0],
		Northing:    coord[1],
		Elevation:   coordinateElevation(coord),
		Code:        code,
		Description: description,
	}
	if existing, ok := state.points[id]; ok {
		if !pointsEqual(existing, pt, compareAttrs) {
			return geom.Point{}, fmt.Errorf("point %q already exists", id)
		}
		return existing, nil
	}
	return pt, nil
}

func importLineStringFeature(state importState, feat feature) (int, int, error) {
	var rawCoords []json.RawMessage
	if err := json.Unmarshal(feat.Geometry.Coordinates, &rawCoords); err != nil {
		return 0, 0, fmt.Errorf("invalid LineString coordinates")
	}
	return importRawLine(state, feat, rawCoords, 0)
}

func importMultiLineStringFeature(state importState, feat feature) (int, int, error) {
	var rawLines [][]json.RawMessage
	if err := json.Unmarshal(feat.Geometry.Coordinates, &rawLines); err != nil {
		return 0, 0, fmt.Errorf("invalid MultiLineString coordinates")
	}
	totalLines := 0
	totalPoints := 0
	for i, rawLine := range rawLines {
		lines, points, err := importRawLine(state, feat, rawLine, i)
		if err != nil {
			return totalLines, totalPoints, err
		}
		totalLines += lines
		totalPoints += points
	}
	return totalLines, totalPoints, nil
}

func importRawLine(state importState, feat feature, rawCoords []json.RawMessage, lineIndex int) (int, int, error) {
	if len(rawCoords) < 2 {
		return 0, 0, fmt.Errorf("LineString requires at least 2 coordinates")
	}
	vertices := make([]geom.Point, 0, len(rawCoords))
	pointCount := 0
	fromID := propertyString(feat.Properties, "from")
	toID := propertyString(feat.Properties, "to")
	for idx, rawCoord := range rawCoords {
		coord, err := parseCoordinate(rawCoord)
		if err != nil {
			return 0, 0, err
		}
		explicitID := ""
		switch {
		case idx == 0:
			explicitID = fromID
		case idx == len(rawCoords)-1:
			explicitID = toID
		}
		if explicitID == "" {
			explicitID = nextPointID(state.points)
		}
		pt, err := ensurePoint(state, explicitID, coord, "", "", false)
		if err != nil {
			return 0, 0, err
		}
		if _, existed := state.points[pt.ID]; !existed {
			pointCount++
		}
		state.points[pt.ID] = pt
		vertices = append(vertices, pt)
	}

	baseID := propertyString(feat.Properties, "id")
	if baseID == "" {
		baseID = featureIDString(feat.ID)
	}
	code := propertyString(feat.Properties, "code")
	description := propertyString(feat.Properties, "description", "desc")
	terrainRole := propertyString(feat.Properties, "terrain_role")
	if terrainRole != "" && terrainRole != "standard" && terrainRole != "ridge" && terrainRole != "drain" {
		return 0, 0, fmt.Errorf("invalid terrain_role %q", terrainRole)
	}

	lineCount := 0
	segments := len(vertices) - 1
	for i := 0; i < segments; i++ {
		id := nextImportedLineID(state.lines, baseID, lineIndex, i, segments)
		state.lines[id] = project.Line{
			ID:          id,
			From:        vertices[i].ID,
			To:          vertices[i+1].ID,
			Code:        code,
			Description: description,
			TerrainRole: terrainRole,
		}
		lineCount++
	}
	return lineCount, pointCount, nil
}

func parseCoordinate(raw json.RawMessage) ([]float64, error) {
	var coord []float64
	if err := json.Unmarshal(raw, &coord); err != nil {
		return nil, fmt.Errorf("invalid coordinate")
	}
	if len(coord) < 2 || len(coord) > 3 {
		return nil, fmt.Errorf("coordinate must have 2 or 3 numbers")
	}
	return coord, nil
}

func coordinateElevation(coord []float64) *float64 {
	if len(coord) < 3 {
		return nil
	}
	v := coord[2]
	return &v
}

func pointsEqual(a, b geom.Point, compareAttrs bool) bool {
	if a.ID != b.ID || a.Easting != b.Easting || a.Northing != b.Northing {
		return false
	}
	if compareAttrs && (a.Code != b.Code || a.Description != b.Description) {
		return false
	}
	switch {
	case a.Elevation == nil && b.Elevation == nil:
		return true
	case a.Elevation == nil || b.Elevation == nil:
		return !compareAttrs
	default:
		return *a.Elevation == *b.Elevation
	}
}

func propertyString(props map[string]any, keys ...string) string {
	for _, key := range keys {
		if props == nil {
			return ""
		}
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
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func featureIDString(value any) string {
	return scalarString(value)
}

func mustMarshal(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func clonePoints(src map[string]geom.Point) map[string]geom.Point {
	dst := make(map[string]geom.Point, len(src))
	for id, pt := range src {
		dst[id] = pt
	}
	return dst
}

func cloneLines(src map[string]project.Line) map[string]project.Line {
	dst := make(map[string]project.Line, len(src))
	for id, line := range src {
		dst[id] = line
	}
	return dst
}

func nextPointID(points map[string]geom.Point) string {
	maxID := 0
	for id := range points {
		n, err := strconv.Atoi(id)
		if err == nil && n > maxID {
			maxID = n
		}
	}
	return strconv.Itoa(maxID + 1)
}

func nextLineID(lines map[string]project.Line) string {
	maxID := 0
	for id := range lines {
		if len(id) < 2 || id[0] != 'L' {
			continue
		}
		n, err := strconv.Atoi(id[1:])
		if err == nil && n > maxID {
			maxID = n
		}
	}
	return fmt.Sprintf("L%d", maxID+1)
}

func nextImportedLineID(lines map[string]project.Line, baseID string, lineIndex, segmentIndex, segments int) string {
	if baseID == "" {
		return nextLineID(lines)
	}
	if segments == 1 && lineIndex == 0 {
		if _, exists := lines[baseID]; !exists {
			return baseID
		}
	}
	candidate := fmt.Sprintf("%s_%d", baseID, lineIndex+segmentIndex+1)
	for {
		if _, exists := lines[candidate]; !exists {
			return candidate
		}
		candidate += "_1"
	}
}
