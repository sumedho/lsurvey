package terrain

import (
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestGenerateContoursFromElevatedPoints(t *testing.T) {
	p := project.New("test")
	z0, z10 := 0.0, 10.0
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0, Elevation: &z0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10, Elevation: &z10}
	p.Points["3"] = geom.Point{ID: "3", Northing: 10, Easting: 0, Elevation: &z10}
	set, err := Generate(p, Options{ID: "C1", Interval: 5})
	if err != nil {
		t.Fatal(err)
	}
	if set.ID != "C1" || set.Interval != 5 {
		t.Fatalf("set=%+v", set)
	}
	if len(set.Polylines) == 0 {
		t.Fatal("expected contour polylines")
	}
	var found bool
	for _, pl := range set.Polylines {
		if pl.Elevation == 5 && len(pl.Vertices) >= 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing elevation 5 contour: %+v", set.Polylines)
	}
}

func TestGenerateContoursValidatesBreaklineElevations(t *testing.T) {
	p := project.New("test")
	z0, z10 := 0.0, 10.0
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0, Elevation: &z0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10}
	p.Points["3"] = geom.Point{ID: "3", Northing: 10, Easting: 0, Elevation: &z10}
	p.Lines["B1"] = project.Line{ID: "B1", From: "1", To: "2"}
	if _, err := Generate(p, Options{ID: "C1", Interval: 5, UseBreakline: true, BreaklineIDs: []string{"B1"}}); err == nil {
		t.Fatal("expected breakline elevation error")
	}
}

func TestGenerateContoursRejectsCrossingBreaklines(t *testing.T) {
	p := project.New("test")
	z := 1.0
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0, Elevation: &z}
	p.Points["2"] = geom.Point{ID: "2", Northing: 10, Easting: 10, Elevation: &z}
	p.Points["3"] = geom.Point{ID: "3", Northing: 10, Easting: 0, Elevation: &z}
	p.Points["4"] = geom.Point{ID: "4", Northing: 0, Easting: 10, Elevation: &z}
	p.Lines["B1"] = project.Line{ID: "B1", From: "1", To: "2"}
	p.Lines["B2"] = project.Line{ID: "B2", From: "3", To: "4"}
	if _, err := Generate(p, Options{ID: "C1", Interval: 1, UseBreakline: true}); err == nil {
		t.Fatal("expected crossing breakline error")
	}
}

func TestInsertConstraintEdgesRecoversBreakline(t *testing.T) {
	points := []vertex{
		{ID: "1", N: 0, E: 0, Z: 0},
		{ID: "2", N: 0, E: 10, Z: 10},
		{ID: "3", N: 10, E: 0, Z: 10},
		{ID: "4", N: 10, E: 10, Z: 20},
	}
	tris, err := triangulate(points)
	if err != nil {
		t.Fatal(err)
	}
	target := normEdge(0, 3)
	if hasEdge(tris, target) {
		t.Skip("delaunay already picked target diagonal")
	}
	got, err := insertConstraintEdges(points, tris, []breakline{{ID: "B1", A: 0, B: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasEdge(got, target) {
		t.Fatalf("missing recovered breakline edge in %+v", got)
	}
}
