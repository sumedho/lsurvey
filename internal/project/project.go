package project

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"lsurvey/internal/geom"
)

const CurrentSchemaVersion = 6

// Line is retained only for decoding project files written before feature geometry.
type Line struct {
	ID          string `json:"id"`
	From        string `json:"from"`
	To          string `json:"to"`
	Code        string `json:"code,omitempty"`
	Description string `json:"description,omitempty"`
	TerrainRole string `json:"terrain_role,omitempty"`
}

const (
	FeatureLine     = "line"
	FeaturePolyline = "polyline"
	FeaturePolygon  = "polygon"
)

type Group struct {
	ID          string `json:"id"`
	Layer       string `json:"layer"`
	Color       int    `json:"color"`
	Description string `json:"description,omitempty"`
}

type Feature struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	PointIDs    []string `json:"point_ids"`
	Code        string   `json:"code,omitempty"`
	Description string   `json:"description,omitempty"`
	TerrainRole string   `json:"terrain_role,omitempty"`
	GroupID     string   `json:"group_id,omitempty"`
}

type FeatureSegment struct {
	FeatureID string
	Index     int
	From      string
	To        string
}

type ContourVertex struct {
	Easting  float64 `json:"easting"`
	Northing float64 `json:"northing"`
}

type ContourPolyline struct {
	ID        string          `json:"id"`
	Elevation float64         `json:"elevation"`
	Index     bool            `json:"index,omitempty"`
	Vertices  []ContourVertex `json:"vertices"`
}

type ContourDiagnostic struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	PointIDs []string `json:"point_ids,omitempty"`
	EdgeIDs  []string `json:"edge_ids,omitempty"`
	Measured float64  `json:"measured,omitempty"`
	Limit    float64  `json:"limit,omitempty"`
}

type ContourGenerationSpec struct {
	Interval       float64  `json:"interval"`
	Base           *float64 `json:"base,omitempty"`
	IndexEvery     int      `json:"index_every,omitempty"`
	IndexEverySet  bool     `json:"index_every_set,omitempty"`
	BreaklineMode  string   `json:"breakline_mode,omitempty"`
	BreaklineIDs   []string `json:"breakline_ids,omitempty"`
	BoundaryCodes  []string `json:"boundary_codes,omitempty"`
	ExclusionCodes []string `json:"exclusion_codes,omitempty"`
	MaxEdge        *float64 `json:"max_edge,omitempty"`
	Smooth         int      `json:"smooth,omitempty"`
}

type ContourSet struct {
	ID               string                 `json:"id"`
	Interval         float64                `json:"interval"`
	Base             float64                `json:"base"`
	IndexEvery       int                    `json:"index_every,omitempty"`
	SourcePoints     []string               `json:"source_points"`
	Breaklines       []string               `json:"breaklines,omitempty"`
	BreaklineRoles   map[string]string      `json:"breakline_roles,omitempty"`
	BoundaryLines    []string               `json:"boundary_lines,omitempty"`
	ExclusionLines   []string               `json:"exclusion_lines,omitempty"`
	Generation       *ContourGenerationSpec `json:"generation,omitempty"`
	Stale            bool                   `json:"stale,omitempty"`
	StaleReason      string                 `json:"stale_reason,omitempty"`
	GeneratedAt      time.Time              `json:"generated_at"`
	TriangleCount    int                    `json:"triangle_count,omitempty"`
	EffectiveMaxEdge float64                `json:"effective_max_edge,omitempty"`
	Diagnostics      []ContourDiagnostic    `json:"diagnostics,omitempty"`
	RawPolylines     []ContourPolyline      `json:"raw_polylines,omitempty"`
	Polylines        []ContourPolyline      `json:"polylines"`
}

type HistoryRecord struct {
	At      time.Time       `json:"at"`
	Command string          `json:"command"`
	Result  string          `json:"result,omitempty"`
	Created []string        `json:"created,omitempty"`
	Updated []string        `json:"updated,omitempty"`
	Error   string          `json:"error,omitempty"`
	Extra   json.RawMessage `json:"extra,omitempty"`
}

type TraverseState struct {
	Start       string   `json:"start"`
	Current     string   `json:"current"`
	Close       string   `json:"close,omitempty"`
	LegPointIDs []string `json:"leg_point_ids"`
}

type GridGroundConversion struct {
	Mode           string  `json:"mode"`
	GridSystem     string  `json:"grid_system,omitempty"`
	AnchorPointID  string  `json:"anchor_point_id,omitempty"`
	AnchorEasting  float64 `json:"anchor_easting"`
	AnchorNorthing float64 `json:"anchor_northing"`
	CSF            float64 `json:"csf"`
}

type Project struct {
	SchemaVersion int                   `json:"schema_version"`
	AppVersion    string                `json:"app_version,omitempty"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	Display       DisplaySettings       `json:"display"`
	Units         map[string]string     `json:"units"`
	Points        map[string]geom.Point `json:"points"`
	Groups        map[string]Group      `json:"groups,omitempty"`
	Features      map[string]Feature    `json:"features,omitempty"`
	LegacyLines   map[string]Line       `json:"lines,omitempty"`
	ContourSets   map[string]ContourSet `json:"contour_sets,omitempty"`
	Traverse      *TraverseState        `json:"traverse,omitempty"`
	GridGround    *GridGroundConversion `json:"grid_ground,omitempty"`
	History       []HistoryRecord       `json:"history"`
}

func New(name string) *Project {
	return &Project{
		SchemaVersion: CurrentSchemaVersion,
		Name:          name,
		Display:       DisplaySettings{Precision: intPtr(DefaultDisplayPrecision)},
		Units: map[string]string{
			"angle":    "dd.mmss",
			"distance": "m",
		},
		Points:      map[string]geom.Point{},
		Groups:      map[string]Group{},
		Features:    map[string]Feature{},
		ContourSets: map[string]ContourSet{},
		History:     []HistoryRecord{},
	}
}

func Load(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.SchemaVersion < 1 || p.SchemaVersion > CurrentSchemaVersion {
		return nil, fmt.Errorf("unsupported schema_version %d", p.SchemaVersion)
	}
	p.ensure()
	return &p, nil
}

func Save(path string, p *Project) error {
	p.ensure()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func (p *Project) AddHistory(command, result string, created []string, err error) {
	p.AddHistoryChange(command, result, created, nil, nil, err)
}

func (p *Project) AddHistoryChange(command, result string, created, updated []string, extra any, err error) {
	r := HistoryRecord{At: time.Now().UTC(), Command: command, Result: result, Created: created, Updated: updated}
	if extra != nil {
		data, marshalErr := json.Marshal(extra)
		if marshalErr == nil {
			r.Extra = data
		}
	}
	if err != nil {
		r.Error = err.Error()
	}
	p.History = append(p.History, r)
}

func (p *Project) SortedPoints() []geom.Point {
	points := make([]geom.Point, 0, len(p.Points))
	for _, pt := range p.Points {
		points = append(points, pt)
	}
	sort.Slice(points, func(i, j int) bool { return PointIDLess(points[i].ID, points[j].ID) })
	return points
}

func PointIDLess(a, b string) bool {
	an, aNumeric := numericPointID(a)
	bn, bNumeric := numericPointID(b)
	switch {
	case aNumeric && bNumeric:
		if an != bn {
			return an < bn
		}
		return a < b
	case aNumeric:
		return true
	case bNumeric:
		return false
	default:
		return a < b
	}
}

func numericPointID(id string) (int, bool) {
	n, err := strconv.Atoi(id)
	return n, err == nil
}

func (p *Project) SortedFeatures() []Feature {
	features := make([]Feature, 0, len(p.Features))
	for _, feature := range p.Features {
		features = append(features, feature)
	}
	sort.Slice(features, func(i, j int) bool { return FeatureIDLess(features[i].ID, features[j].ID) })
	return features
}

func FeatureIDLess(a, b string) bool {
	aPrefix, an, aNumeric := numericSuffixID(a)
	bPrefix, bn, bNumeric := numericSuffixID(b)
	if aNumeric && bNumeric && aPrefix == bPrefix {
		if an != bn {
			return an < bn
		}
		return a < b
	}
	return a < b
}

func numericSuffixID(id string) (string, int, bool) {
	end := len(id)
	start := end
	for start > 0 && id[start-1] >= '0' && id[start-1] <= '9' {
		start--
	}
	if start == end {
		return id, 0, false
	}
	n, err := strconv.Atoi(id[start:])
	return id[:start], n, err == nil
}

func (p *Project) NextPointID() string {
	maxID := 0
	for id := range p.Points {
		n, err := strconv.Atoi(id)
		if err == nil && n > maxID {
			maxID = n
		}
	}
	return strconv.Itoa(maxID + 1)
}

func (p *Project) NextFeatureID() string {
	maxID := 0
	for id := range p.Features {
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

func (p *Project) SortedGroups() []Group {
	groups := make([]Group, 0, len(p.Groups))
	for _, group := range p.Groups {
		groups = append(groups, group)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
	return groups
}

func (p *Project) FeatureSegments(feature Feature) []FeatureSegment {
	if len(feature.PointIDs) < 2 {
		return nil
	}
	out := make([]FeatureSegment, 0, len(feature.PointIDs))
	for i := 1; i < len(feature.PointIDs); i++ {
		out = append(out, FeatureSegment{FeatureID: feature.ID, Index: i - 1, From: feature.PointIDs[i-1], To: feature.PointIDs[i]})
	}
	if feature.Kind == FeaturePolygon {
		out = append(out, FeatureSegment{FeatureID: feature.ID, Index: len(feature.PointIDs) - 1, From: feature.PointIDs[len(feature.PointIDs)-1], To: feature.PointIDs[0]})
	}
	return out
}

func (p *Project) ensure() {
	p.SchemaVersion = CurrentSchemaVersion
	if p.Units == nil {
		p.Units = map[string]string{"angle": "dd.mmss", "distance": "m"}
	}
	if p.Display.Precision == nil {
		p.Display.Precision = intPtr(DefaultDisplayPrecision)
	} else {
		p.Display.Precision = intPtr(clampPrecision(*p.Display.Precision))
	}
	if p.Points == nil {
		p.Points = map[string]geom.Point{}
	}
	if p.Groups == nil {
		p.Groups = map[string]Group{}
	}
	if p.Features == nil {
		p.Features = map[string]Feature{}
	}
	for id, line := range p.LegacyLines {
		if _, exists := p.Features[id]; !exists {
			p.Features[id] = Feature{
				ID: id, Kind: FeatureLine, PointIDs: []string{line.From, line.To},
				Code: line.Code, Description: line.Description, TerrainRole: line.TerrainRole,
			}
		}
	}
	p.LegacyLines = nil
	if p.ContourSets == nil {
		p.ContourSets = map[string]ContourSet{}
	}
	if p.History == nil {
		p.History = []HistoryRecord{}
	}
}

func (p *Project) SortedContourSets() []ContourSet {
	sets := make([]ContourSet, 0, len(p.ContourSets))
	for _, set := range p.ContourSets {
		sets = append(sets, set)
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].ID < sets[j].ID })
	return sets
}

func (p *Project) MarkContoursStale(reason string) {
	for id, set := range p.ContourSets {
		if set.Generation == nil {
			continue
		}
		set.Stale = true
		set.StaleReason = reason
		p.ContourSets[id] = set
	}
}

func (p *Project) CoordinateLabel() string {
	if p.GridGround == nil {
		return ""
	}
	return p.GridGround.GridSystem
}
