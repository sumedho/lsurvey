package csvpoints

import (
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestImportCSVStrictHeaderAndRows(t *testing.T) {
	p := project.New("test")
	input := "id,easting,northing,elevation,code,description\n1,200,100,5.5,PEG,corner\n2,210,110,,TREE,\n"
	count, err := Import(strings.NewReader(input), p)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count=%d want 2", count)
	}
	if p.Points["1"].Code != "PEG" || p.Points["1"].Elevation == nil || *p.Points["1"].Elevation != 5.5 {
		t.Fatalf("point 1=%+v", p.Points["1"])
	}
	if p.Points["2"].Elevation != nil {
		t.Fatalf("point 2 elevation=%v want nil", p.Points["2"].Elevation)
	}
}

func TestImportCSVRejectsBadHeader(t *testing.T) {
	p := project.New("test")
	if _, err := Import(strings.NewReader("point,northing,easting,elevation,code,description\n1,2,3,,,x\n"), p); err == nil {
		t.Fatal("expected header error")
	}
}

func TestImportCSVIsAtomicOnRowError(t *testing.T) {
	p := project.New("test")
	p.Points["1"] = geom.Point{ID: "1", Northing: 1, Easting: 1, Code: "OLD"}
	input := "id,easting,northing,elevation,code,description\n1,200,100,,NEW,\n2,bad,210,,TREE,\n"
	count, err := Import(strings.NewReader(input), p)
	if err == nil {
		t.Fatal("expected row error")
	}
	if count != 1 {
		t.Fatalf("count=%d want 1 rows read before error", count)
	}
	got := p.Points["1"]
	if got.Northing != 1 || got.Easting != 1 || got.Code != "OLD" {
		t.Fatalf("point was partially imported: %+v", got)
	}
	if _, ok := p.Points["2"]; ok {
		t.Fatal("point 2 should not be imported after row error")
	}
}

func TestExportCSV(t *testing.T) {
	p := project.New("test")
	z := 5.5
	p.Points["2"] = geom.Point{ID: "2", Northing: 110, Easting: 210}
	p.Points["1"] = geom.Point{ID: "1", Northing: 100, Easting: 200, Elevation: &z, Code: "PEG", Description: "corner"}
	var out strings.Builder
	if err := Export(&out, p); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"id,easting,northing,elevation,code,description\n",
		"1,200,100,5.5,PEG,corner\n",
		"2,210,110,,,\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("csv missing %q:\n%s", want, got)
		}
	}
}
