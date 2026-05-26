package landxml

import (
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestWriteExportsPointsFeatureSegmentsAndGroupMetadata(t *testing.T) {
	p := project.New("test")
	p.Groups["BND"] = project.Group{ID: "BND", Layer: "BOUNDARY", Color: 1}
	p.Points["1"] = geom.Point{ID: "1", Easting: 100, Northing: 200, GroupID: "BND"}
	p.Points["2"] = geom.Point{ID: "2", Easting: 110, Northing: 200}
	p.Points["3"] = geom.Point{ID: "3", Easting: 110, Northing: 210}
	p.Features["LOT"] = project.Feature{ID: "LOT", Kind: project.FeaturePolygon, PointIDs: []string{"1", "2", "3"}, GroupID: "BND"}
	var out strings.Builder
	if err := Write(&out, p); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{`<LandXML`, `linearUnit="meter"`, `<CgPoint name="1">200 100 0`, `<PlanFeature name="LOT">`, `pntRef="3">210 110 0`, `pntRef="1">200 100 0`, `label="layer" value="BOUNDARY"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("LandXML missing %q:\n%s", want, got)
		}
	}
}

func TestWriteRejectsNonMetricProject(t *testing.T) {
	p := project.New("test")
	p.Units["distance"] = "ft"
	if err := Write(&strings.Builder{}, p); err == nil {
		t.Fatal("expected metric-only export error")
	}
}
