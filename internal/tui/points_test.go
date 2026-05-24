package tui

import (
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestFilterAndSortPoints(t *testing.T) {
	p := project.New("test")
	p.Points["2"] = geom.Point{ID: "2", Northing: 20, Easting: 1, Code: "TREE", Description: "large"}
	p.Points["1"] = geom.Point{ID: "1", Northing: 10, Easting: 2, Code: "PEG", Description: "corner"}
	p.Points["3"] = geom.Point{ID: "3", Northing: 30, Easting: 3, Code: "PEG", Description: "side"}

	filtered := FilterAndSortPoints(p, "peg", SortNorth, false)
	if len(filtered) != 2 {
		t.Fatalf("len=%d want 2", len(filtered))
	}
	if filtered[0].ID != "3" || filtered[1].ID != "1" {
		t.Fatalf("order=%s,%s want 3,1", filtered[0].ID, filtered[1].ID)
	}
}

func TestParseSortField(t *testing.T) {
	if got, ok := ParseSortField("code"); !ok || got != SortCode {
		t.Fatalf("got %q %v", got, ok)
	}
	if _, ok := ParseSortField("bad"); ok {
		t.Fatal("bad sort field accepted")
	}
}

func TestFormatPointRowsUsesThreeDecimalPlaces(t *testing.T) {
	z := 3.4567
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 1.23456, Easting: 2.34567, Elevation: &z}
	got := FormatPointRows(p.SortedPoints(), 3)
	for _, want := range []string{"1.235", "2.346", "3.457"} {
		if !strings.Contains(got, want) {
			t.Fatalf("point rows missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "1.2346") || strings.Contains(got, "3.4567") {
		t.Fatalf("point rows should not show four decimal places:\n%s", got)
	}
}

func TestFormatPointRowsUsesConfiguredPrecision(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 1.23456, Easting: 2.34567}
	got := FormatPointRows(p.SortedPoints(), 1)
	if !strings.Contains(got, "1.2") || !strings.Contains(got, "2.3") {
		t.Fatalf("point rows missing one decimal values:\n%s", got)
	}
	if strings.Contains(got, "1.235") {
		t.Fatalf("point rows should use configured precision:\n%s", got)
	}
}

func TestFormatLineRowsIncludesContoursWithoutHeightClipping(t *testing.T) {
	lines := []project.Line{
		{ID: "L1", From: "1", To: "2", Code: "BOUNDARY", Description: "edge"},
	}
	contours := []project.ContourSet{
		{ID: "C1", Interval: 0.5, Polylines: []project.ContourPolyline{{ID: "PL1"}}},
	}
	got := FormatLineRows(lines, contours, 2)
	for _, want := range []string{"L1", "Contours:", "C1", "0.50"} {
		if !strings.Contains(got, want) {
			t.Fatalf("line rows missing %q:\n%s", want, got)
		}
	}
}
