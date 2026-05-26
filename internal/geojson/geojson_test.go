package geojson

import (
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestExportGeoJSONPointsAndLines(t *testing.T) {
	p := project.New("test")
	z1 := 5.5
	z2 := 6.5
	p.Points["1"] = geom.Point{ID: "1", Easting: 200, Northing: 100, Elevation: &z1, Code: "PEG", Description: "corner"}
	p.Points["2"] = geom.Point{ID: "2", Easting: 210, Northing: 110, Elevation: &z2}
	p.Features["L1"] = project.Feature{ID: "L1", Kind: project.FeatureLine, PointIDs: []string{"1", "2"}, Code: "BOUNDARY", Description: "edge", TerrainRole: "ridge"}

	var out strings.Builder
	if err := Export(&out, p); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		`"type": "FeatureCollection"`,
		`"feature_type": "point"`,
		`"feature_type": "line"`,
		`"coordinates": [`,
		`200,`,
		`100,`,
		`5.5`,
		`"id": "1"`,
		`"code": "PEG"`,
		`"description": "corner"`,
		`"point_ids": "1,2"`,
		`"terrain_role": "ridge"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("geojson missing %q:\n%s", want, got)
		}
	}
}

func TestExportGeoJSONPreservesMixedElevationVertices(t *testing.T) {
	p := project.New("test")
	z := 5.5
	p.Points["1"] = geom.Point{ID: "1", Easting: 0, Northing: 0, Elevation: &z}
	p.Points["2"] = geom.Point{ID: "2", Easting: 10, Northing: 0}
	p.Features["L1"] = project.Feature{ID: "L1", Kind: project.FeatureLine, PointIDs: []string{"1", "2"}}

	var out strings.Builder
	if err := Export(&out, p); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "5.5") {
		t.Fatalf("mixed dimensional feature should retain elevated vertex:\n%s", got)
	}
}

func TestImportGeoJSONPointsAndProperties(t *testing.T) {
	p := project.New("test")
	input := `{
  "type":"FeatureCollection",
  "features":[
    {"type":"Feature","geometry":{"type":"Point","coordinates":[200,100,5.5]},"properties":{"id":"1","code":"PEG","description":"corner"}},
    {"type":"Feature","geometry":{"type":"Point","coordinates":[210,110]},"properties":{"id":"2","code":"TREE","desc":"plant"}}
  ]
}`
	points, lines, err := Import(strings.NewReader(input), p)
	if err != nil {
		t.Fatal(err)
	}
	if points != 2 || lines != 0 {
		t.Fatalf("points=%d lines=%d", points, lines)
	}
	if p.Points["1"].Elevation == nil || *p.Points["1"].Elevation != 5.5 {
		t.Fatalf("point1=%+v", p.Points["1"])
	}
	if p.Points["2"].Elevation != nil {
		t.Fatalf("point2 elevation=%v want nil", p.Points["2"].Elevation)
	}
	if p.Points["2"].Description != "plant" {
		t.Fatalf("description=%q want plant", p.Points["2"].Description)
	}
}

func TestImportGeoJSONRejectsPointIDCollision(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Easting: 1, Northing: 1, Code: "OLD"}
	input := `{
  "type":"FeatureCollection",
  "features":[
    {"type":"Feature","geometry":{"type":"Point","coordinates":[200,100]},"properties":{"id":"1","code":"NEW"}}
  ]
}`
	points, lines, err := Import(strings.NewReader(input), p)
	if err == nil {
		t.Fatal("expected collision error")
	}
	if points != 0 || lines != 0 {
		t.Fatalf("points=%d lines=%d", points, lines)
	}
	if got := p.Points["1"]; got.Easting != 1 || got.Northing != 1 || got.Code != "OLD" {
		t.Fatalf("point overwritten: %+v", got)
	}
}

func TestImportGeoJSONLineStringPreservesPolyline(t *testing.T) {
	p := project.New("test")
	input := `{
  "type":"FeatureCollection",
  "features":[
    {"type":"Feature","geometry":{"type":"LineString","coordinates":[[0,0],[10,0,2],[10,10]]},"properties":{"id":"BND","code":"BOUNDARY","description":"edge"}}
  ]
}`
	points, lines, err := Import(strings.NewReader(input), p)
	if err != nil {
		t.Fatal(err)
	}
	if points != 3 || lines != 1 {
		t.Fatalf("points=%d lines=%d want 3/1", points, lines)
	}
	if got := p.Features["BND"]; got.Kind != project.FeaturePolyline || len(got.PointIDs) != 3 {
		t.Fatalf("features=%+v want BND polyline", p.Features)
	}
	if p.Points["2"].Elevation == nil || *p.Points["2"].Elevation != 2 {
		t.Fatalf("point2=%+v", p.Points["2"])
	}
}

func TestImportGeoJSONPolygonAndAutoIDs(t *testing.T) {
	p := project.New("test")
	p.Features["L1"] = project.Feature{ID: "L1", Kind: project.FeatureLine, PointIDs: []string{"1", "2"}}
	input := `{
  "type":"FeatureCollection",
  "features":[
    {"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]},"properties":{"code":"FENCE"}}
  ]
}`
	points, lines, err := Import(strings.NewReader(input), p)
	if err != nil {
		t.Fatal(err)
	}
	if points != 3 || lines != 1 {
		t.Fatalf("points=%d lines=%d want 3/1", points, lines)
	}
	if got := p.Features["L2"]; got.Kind != project.FeaturePolygon {
		t.Fatalf("features=%+v want polygon L2", p.Features)
	}
}

func TestGeoJSONRoundTripPreservesLinePointReferences(t *testing.T) {
	source := project.New("test")
	source.Points["1"] = geom.Point{ID: "1", Easting: 0, Northing: 0}
	source.Points["2"] = geom.Point{ID: "2", Easting: 10, Northing: 0}
	source.Features["L1"] = project.Feature{ID: "L1", Kind: project.FeatureLine, PointIDs: []string{"1", "2"}, Code: "BOUNDARY", TerrainRole: "drain"}

	var out strings.Builder
	if err := Export(&out, source); err != nil {
		t.Fatal(err)
	}

	dest := project.New("dest")
	points, lines, err := Import(strings.NewReader(out.String()), dest)
	if err != nil {
		t.Fatal(err)
	}
	if points != 2 || lines != 1 {
		t.Fatalf("points=%d lines=%d want 2/1", points, lines)
	}
	if len(dest.Points) != 2 {
		t.Fatalf("point count=%d want 2", len(dest.Points))
	}
	if got := dest.Features["L1"]; got.PointIDs[0] != "1" || got.PointIDs[1] != "2" {
		t.Fatalf("line=%+v want from=1 to=2", got)
	}
	if dest.Features["L1"].TerrainRole != "drain" {
		t.Fatalf("terrain role=%q want drain", dest.Features["L1"].TerrainRole)
	}
}

func TestImportGeoJSONIsAtomic(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Easting: 1, Northing: 1, Code: "OLD"}
	input := `{
  "type":"FeatureCollection",
  "features":[
    {"type":"Feature","geometry":{"type":"Point","coordinates":[200,100]},"properties":{"id":"2","code":"NEW"}},
    {"type":"Feature","geometry":{"type":"Polygon","coordinates":[]},"properties":{"id":"3"}}
  ]
}`
	points, lines, err := Import(strings.NewReader(input), p)
	if err == nil {
		t.Fatal("expected import error")
	}
	if points != 0 || lines != 0 {
		t.Fatalf("points=%d lines=%d", points, lines)
	}
	if len(p.Points) != 1 || p.Points["1"].Code != "OLD" {
		t.Fatalf("project mutated: %+v", p.Points)
	}
	if len(p.Features) != 0 {
		t.Fatalf("features mutated: %+v", p.Features)
	}
}
