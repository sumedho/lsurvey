package tui

import (
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestRenderMapNoPoints(t *testing.T) {
	got := renderMap(project.New("test"), newMapState(), 60, 18, 3)
	if !strings.Contains(got, "No points to map") {
		t.Fatalf("map missing empty state:\n%s", got)
	}
}

func TestRenderMapOnePointCenteredAndLabeled(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 100, Easting: 200}
	got := renderMap(p, newMapState(), 60, 18, 3)
	if !strings.Contains(got, "*1") {
		t.Fatalf("map missing point label:\n%s", got)
	}
}

func TestVisibleBoundsZoom(t *testing.T) {
	fit := mapBounds{MinE: 0, MaxE: 100, MinN: 0, MaxN: 100}
	base := visibleBounds(fit, MapState{Zoom: 1})
	zoomed := visibleBounds(fit, MapState{Zoom: 2, CenterE: 50, CenterN: 50, Custom: true})
	if zoomed.MaxE-zoomed.MinE >= base.MaxE-base.MinE {
		t.Fatalf("zoomed span=%f base span=%f", zoomed.MaxE-zoomed.MinE, base.MaxE-base.MinE)
	}
}

func TestMapLineOverlay(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 10}
	p.Lines["L1"] = project.Line{ID: "L1", From: "1", To: "2"}
	got := renderMap(p, MapState{ShowLines: true, Zoom: 1}, 60, 18, 3)
	if !strings.Contains(got, ".") {
		t.Fatalf("map missing line overlay:\n%s", got)
	}
}

func TestRenderMapSkipsPointsOutsideCustomView(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 100, Easting: 100}
	state := MapState{Zoom: 10, CenterE: 0, CenterN: 0, Custom: true}
	got := renderMap(p, state, 60, 18, 3)
	if !strings.Contains(got, "*1") {
		t.Fatalf("map missing visible point:\n%s", got)
	}
	if strings.Contains(got, "*2") {
		t.Fatalf("map should not show off-screen point:\n%s", got)
	}
}

func TestRenderMapClipsLinesToCustomView(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: -100}
	p.Points["2"] = geom.Point{ID: "2", Northing: 0, Easting: 100}
	p.Lines["L1"] = project.Line{ID: "L1", From: "1", To: "2"}
	state := MapState{ShowLines: true, Zoom: 10, CenterE: 0, CenterN: 0, Custom: true}
	got := renderMap(p, state, 60, 18, 3)
	if !strings.Contains(got, ".") {
		t.Fatalf("map should show clipped crossing line:\n%s", got)
	}
	if strings.Contains(got, "*1") || strings.Contains(got, "*2") {
		t.Fatalf("map should not clamp off-screen endpoints:\n%s", got)
	}
}

func TestMapStateFitAndZoom(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 10, Easting: 10}
	state := newMapState()
	state.zoomBy(p, 1.5)
	if state.Zoom != 1.5 || !state.Custom {
		t.Fatalf("state=%+v", state)
	}
	state.fit()
	if state.Zoom != 1 || state.Custom {
		t.Fatalf("state=%+v", state)
	}
}

func TestMapStatePan(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 0, Easting: 0}
	p.Points["2"] = geom.Point{ID: "2", Northing: 100, Easting: 100}
	state := newMapState()
	state.panBy(p, 0.2, -0.2)
	if !state.Custom {
		t.Fatal("pan should create custom map view")
	}
	if state.CenterE <= 50 {
		t.Fatalf("center easting=%f want > 50", state.CenterE)
	}
	if state.CenterN >= 50 {
		t.Fatalf("center northing=%f want < 50", state.CenterN)
	}
}
