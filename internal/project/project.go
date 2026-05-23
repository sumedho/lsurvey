package project

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"lsurvey/internal/geom"
)

const CurrentSchemaVersion = 2

type Line struct {
	ID          string `json:"id"`
	From        string `json:"from"`
	To          string `json:"to"`
	Code        string `json:"code,omitempty"`
	Description string `json:"description,omitempty"`
}

type ContourVertex struct {
	Northing float64 `json:"northing"`
	Easting  float64 `json:"easting"`
}

type ContourPolyline struct {
	ID        string          `json:"id"`
	Elevation float64         `json:"elevation"`
	Index     bool            `json:"index,omitempty"`
	Vertices  []ContourVertex `json:"vertices"`
}

type ContourSet struct {
	ID           string            `json:"id"`
	Interval     float64           `json:"interval"`
	Base         float64           `json:"base"`
	IndexEvery   int               `json:"index_every,omitempty"`
	SourcePoints []string          `json:"source_points"`
	Breaklines   []string          `json:"breaklines,omitempty"`
	GeneratedAt  time.Time         `json:"generated_at"`
	Polylines    []ContourPolyline `json:"polylines"`
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

type Project struct {
	SchemaVersion int                   `json:"schema_version"`
	Name          string                `json:"name"`
	Description   string                `json:"description,omitempty"`
	Display       DisplaySettings       `json:"display"`
	Units         map[string]string     `json:"units"`
	Points        map[string]geom.Point `json:"points"`
	Lines         map[string]Line       `json:"lines"`
	ContourSets   map[string]ContourSet `json:"contour_sets,omitempty"`
	Traverse      *TraverseState        `json:"traverse,omitempty"`
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
		Lines:       map[string]Line{},
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
	if p.SchemaVersion != CurrentSchemaVersion && p.SchemaVersion != 1 {
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
	r := HistoryRecord{At: time.Now().UTC(), Command: command, Result: result, Created: created}
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
	sort.Slice(points, func(i, j int) bool { return points[i].ID < points[j].ID })
	return points
}

func (p *Project) SortedLines() []Line {
	lines := make([]Line, 0, len(p.Lines))
	for _, line := range p.Lines {
		lines = append(lines, line)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].ID < lines[j].ID })
	return lines
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
	if p.Lines == nil {
		p.Lines = map[string]Line{}
	}
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
