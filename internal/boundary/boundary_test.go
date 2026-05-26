package boundary

import (
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestForPolygonReportsSummaryLegsAndInteriorLabelPoint(t *testing.T) {
	p := project.New("test")
	for id, xy := range map[string][2]float64{
		"1": {0, 0}, "2": {4, 0}, "3": {4, 4}, "4": {2, 2}, "5": {0, 4},
	} {
		p.Points[id] = geom.Point{ID: id, Easting: xy[0], Northing: xy[1]}
	}
	p.Features["LOT1"] = project.Feature{ID: "LOT1", Kind: project.FeaturePolygon, PointIDs: []string{"1", "2", "3", "4", "5"}, Code: "LOT"}

	got, err := ForPolygon(p, "LOT1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Area != 12 || got.Perimeter == 0 || len(got.Legs) != 5 || got.Legs[4].From != "5" || got.Legs[4].To != "1" {
		t.Fatalf("schedule=%+v", got)
	}
	point := LabelPoint(p, got.Feature)
	if !ContainsPoint(p, got.Feature, point) {
		t.Fatalf("label point=%+v should be inside polygon", point)
	}
}

func TestWriteCSVExportsAllPolygonLegsInFeatureOrder(t *testing.T) {
	p := project.New("test")
	for id, xy := range map[string][2]float64{"1": {0, 0}, "2": {2, 0}, "3": {0, 2}, "4": {4, 0}, "5": {6, 0}, "6": {4, 2}} {
		p.Points[id] = geom.Point{ID: id, Easting: xy[0], Northing: xy[1]}
	}
	p.Features["P2"] = project.Feature{ID: "P2", Kind: project.FeaturePolygon, PointIDs: []string{"4", "5", "6"}}
	p.Features["P1"] = project.Feature{ID: "P1", Kind: project.FeaturePolygon, PointIDs: []string{"1", "2", "3"}, Code: "LOT", Description: "first"}

	var out strings.Builder
	if err := WriteCSV(&out, p, ""); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "polygon_id,polygon_code,polygon_description,area,perimeter,leg,from,to,bearing,distance\n") {
		t.Fatalf("header missing:\n%s", got)
	}
	if strings.Index(got, "P1,LOT,first") > strings.Index(got, "P2,,") || strings.Count(got, "\n") != 7 {
		t.Fatalf("rows/order incorrect:\n%s", got)
	}
}
