package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func TestSessionTracksOnlyMutatingCogoCommands(t *testing.T) {
	s := NewSession(project.New("test"), "", "test-version")
	outcome, err := s.Execute("pt add 1 0 0")
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.ProjectChanged || !s.Dirty || len(s.Project.History) != 1 {
		t.Fatalf("outcome=%+v dirty=%v history=%d", outcome, s.Dirty, len(s.Project.History))
	}
	if _, err := s.Execute("pt add 2 10 0"); err != nil {
		t.Fatal(err)
	}
	s.Dirty = false
	historyCount := len(s.Project.History)
	outcome, err = s.Execute("inverse 1 2")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.ProjectChanged || s.Dirty || len(s.Project.History) != historyCount {
		t.Fatalf("outcome=%+v dirty=%v history=%d want %d", outcome, s.Dirty, len(s.Project.History), historyCount)
	}
}

func TestSessionProjectLifecycleAndMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "job")
	s := NewSession(project.New("first"), "", "v1")
	outcome, err := s.Execute("desc Boundary survey")
	if err != nil || !outcome.ProjectChanged || !s.Dirty {
		t.Fatalf("desc outcome=%+v dirty=%v err=%v", outcome, s.Dirty, err)
	}
	if _, err := s.Execute("save " + path); err != nil {
		t.Fatal(err)
	}
	if s.Path != path+".srv" || s.Dirty {
		t.Fatalf("path=%q dirty=%v", s.Path, s.Dirty)
	}
	outcome, err = s.Execute("new second")
	if err != nil || !outcome.ProjectReplaced || s.Project.Name != "second" {
		t.Fatalf("new outcome=%+v project=%+v err=%v", outcome, s.Project, err)
	}
	outcome, err = s.Execute("open " + path)
	if err != nil || !outcome.ProjectReplaced || s.Project.Description != "Boundary survey" {
		t.Fatalf("open outcome=%+v project=%+v err=%v", outcome, s.Project, err)
	}
}

func TestSessionRoutesImportAndExport(t *testing.T) {
	dir := t.TempDir()
	s := NewSession(project.New("source"), "", "v1")
	if _, err := s.Execute("pt add 1 100 200 PEG"); err != nil {
		t.Fatal(err)
	}
	csvPath := filepath.Join(dir, "points")
	outcome, err := s.Execute("export csv " + csvPath)
	if err != nil || !strings.Contains(outcome.Message, ".csv") {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	dest := NewSession(project.New("dest"), "", "v1")
	outcome, err = dest.Execute("import csv " + csvPath)
	if err != nil || !outcome.ProjectChanged || dest.Project.Points["1"].Code != "PEG" {
		t.Fatalf("outcome=%+v project=%+v err=%v", outcome, dest.Project, err)
	}
	if _, err := os.Stat(csvPath + ".csv"); err != nil {
		t.Fatal(err)
	}
	xmlPath := filepath.Join(dir, "geometry")
	outcome, err = s.Execute("export landxml " + xmlPath)
	if err != nil || !strings.Contains(outcome.Message, ".xml") {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if _, err := os.Stat(xmlPath + ".xml"); err != nil {
		t.Fatal(err)
	}
}

func TestSessionRoutesBoundarySchedulesAndCodeLibraries(t *testing.T) {
	dir := t.TempDir()
	source := NewSession(project.New("source"), "", "v1")
	for _, command := range []string{
		"group add PEGS layer=PEG_MARKS color=1",
		"code style set PEG group=PEGS",
		"pt add 1 0 0 PEG", "pt add 2 10 0", "pt add 3 0 10",
		"polygon add LOT1 1 2 3",
	} {
		if _, err := source.Execute(command); err != nil {
			t.Fatal(err)
		}
	}
	schedulePath := filepath.Join(dir, "boundaries")
	if outcome, err := source.Execute("export boundarycsv " + schedulePath + " polygon=LOT1"); err != nil || !strings.Contains(outcome.Message, ".csv") {
		t.Fatalf("boundary outcome=%+v err=%v", outcome, err)
	}
	if _, err := os.Stat(schedulePath + ".csv"); err != nil {
		t.Fatal(err)
	}
	libraryPath := filepath.Join(dir, "survey")
	if _, err := source.Execute("export codes " + libraryPath); err != nil {
		t.Fatal(err)
	}
	dest := NewSession(project.New("dest"), "", "v1")
	outcome, err := dest.Execute("import codes " + libraryPath)
	if err != nil || !outcome.ProjectChanged || dest.Project.PointCodeStyles["PEG"] != "PEGS" {
		t.Fatalf("codes outcome=%+v project=%+v err=%v", outcome, dest.Project, err)
	}
	if _, err := dest.Execute("undo"); err != nil || len(dest.Project.PointCodeStyles) != 0 {
		t.Fatalf("library import should be undoable: styles=%+v err=%v", dest.Project.PointCodeStyles, err)
	}
	if _, err := dest.Execute("redo"); err != nil || dest.Project.PointCodeStyles["PEG"] != "PEGS" {
		t.Fatalf("library import should be redoable: styles=%+v err=%v", dest.Project.PointCodeStyles, err)
	}
	if _, err := dest.Execute("pt add 1 0 0 PEG"); err != nil || dest.Project.Points["1"].GroupID != "PEGS" {
		t.Fatalf("imported defaults not applied: %+v err=%v", dest.Project.Points["1"], err)
	}
}

func TestSessionFailedCommandDoesNotRecordHistory(t *testing.T) {
	s := NewSession(project.New("test"), "", "v1")
	if _, err := s.Execute("inverse missing other"); err == nil {
		t.Fatal("expected command error")
	}
	if _, err := s.Execute("units distance=ft malformed"); err == nil {
		t.Fatal("expected units error")
	}
	if got := s.Project.Units["distance"]; got != "m" {
		t.Fatalf("failed units command changed distance to %q", got)
	}
	if s.Dirty || len(s.Project.History) != 0 {
		t.Fatalf("dirty=%v history=%d", s.Dirty, len(s.Project.History))
	}
}

func TestSessionSeparatesLifecycleAndCOGORouting(t *testing.T) {
	s := NewSession(project.New("test"), "", "v1")
	outcome, err := s.Execute("desc inverse missing other")
	if err != nil || !outcome.ProjectChanged || s.Project.Description != "inverse missing other" {
		t.Fatalf("lifecycle desc outcome=%+v description=%q err=%v", outcome, s.Project.Description, err)
	}
	if _, err := s.Execute("definitely-unknown"); err == nil || !strings.Contains(err.Error(), `unknown command "definitely-unknown"`) {
		t.Fatalf("unknown command should fall through to cogo: %v", err)
	}
}

func TestSessionUndoRedoRestoresDataAndAppendsAudit(t *testing.T) {
	s := NewSession(project.New("test"), "", "v1")
	if _, err := s.Execute("pt add 1 100 200 PEG"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("pt edit 1 code=MARK"); err != nil {
		t.Fatal(err)
	}
	if got := s.Project.History[1].Updated; len(got) != 1 || got[0] != "point:1" {
		t.Fatalf("updated=%v want point:1", got)
	}

	outcome, err := s.Execute("undo")
	if err != nil || !outcome.ProjectChanged || s.Project.Points["1"].Code != "PEG" {
		t.Fatalf("undo outcome=%+v point=%+v err=%v", outcome, s.Project.Points["1"], err)
	}
	if len(s.Project.History) != 3 || s.Project.History[2].Command != "undo" || !strings.Contains(string(s.Project.History[2].Extra), `"target_command":"pt edit 1 code=MARK"`) {
		t.Fatalf("undo history=%+v", s.Project.History)
	}

	outcome, err = s.Execute("redo")
	if err != nil || !outcome.ProjectChanged || s.Project.Points["1"].Code != "MARK" {
		t.Fatalf("redo outcome=%+v point=%+v err=%v", outcome, s.Project.Points["1"], err)
	}
	if len(s.Project.History) != 4 || s.Project.History[3].Command != "redo" {
		t.Fatalf("redo history=%+v", s.Project.History)
	}
}

func TestSessionDeleteRecordsDeletedAudit(t *testing.T) {
	s := NewSession(project.New("test"), "", "v1")
	if _, err := s.Execute("pt add 1 100 200 PEG"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("pt del 1"); err != nil {
		t.Fatal(err)
	}
	record := s.Project.History[1]
	if len(record.Deleted) != 1 || record.Deleted[0] != "point:1" {
		t.Fatalf("deleted=%v want point:1", record.Deleted)
	}
	if len(record.Updated) != 0 {
		t.Fatalf("updated=%v want none", record.Updated)
	}
	got, err := s.Execute("history info 2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Message, "deleted=point:1") || strings.Contains(got.Message, "updated=point:1") {
		t.Fatalf("history detail=%q", got.Message)
	}
}

func TestSessionUndoLifecycleAndRedoInvalidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "job")
	s := NewSession(project.New("test"), "", "v1")
	if _, err := s.Execute("pt add 1 0 0"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("save " + path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("undo"); err != nil {
		t.Fatal("save should retain undo:", err)
	}
	if _, err := s.Execute("pt add 2 0 0"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("redo"); err == nil {
		t.Fatal("new edit after undo should clear redo")
	}
	if _, err := s.Execute("new second"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("undo"); err == nil {
		t.Fatal("new should clear undo")
	}
	if _, err := s.Execute("open " + path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("undo"); err == nil {
		t.Fatal("open should clear undo")
	}
}

func TestSessionUndoIncludesImportsAndMetadata(t *testing.T) {
	dir := t.TempDir()
	source := NewSession(project.New("source"), "", "v1")
	if _, err := source.Execute("pt add 1 100 200"); err != nil {
		t.Fatal(err)
	}
	csvPath := filepath.Join(dir, "points")
	if _, err := source.Execute("export csv " + csvPath); err != nil {
		t.Fatal(err)
	}
	s := NewSession(project.New("target"), "", "v1")
	if _, err := s.Execute("desc Before undo"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("precision 4"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("import csv " + csvPath); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute("undo"); err != nil {
		t.Fatal(err)
	}
	if len(s.Project.Points) != 0 {
		t.Fatalf("imported points survived undo: %+v", s.Project.Points)
	}
	if _, err := s.Execute("undo"); err != nil {
		t.Fatal(err)
	}
	if s.Project.DisplayPrecision() != 3 {
		t.Fatalf("precision=%d want default after undo", s.Project.DisplayPrecision())
	}
}

func TestSessionCommitConvertedPointsAssignsCRSStylesAndSupportsUndo(t *testing.T) {
	p := project.New("converted")
	p.Groups["PEGS"] = project.Group{ID: "PEGS", Layer: "PEGS", Color: 1}
	p.PointCodeStyles["PEG"] = "PEGS"
	s := NewSession(p, "", "v1")
	outcome, err := s.CommitConvertedPoints(ConversionCommit{
		Points:       []geom.Point{{ID: "1", Easting: 500000, Northing: 6500000, Code: "PEG"}},
		TargetCRS:    project.HorizontalCRS{Datum: "GDA2020", Projection: "MGA", Zone: 50},
		SourceSystem: "MGA94", TargetSystem: "MGA2020", Model: "conformal",
	})
	if err != nil || !outcome.ProjectChanged || p.Points["1"].GroupID != "PEGS" {
		t.Fatalf("outcome=%+v point=%+v err=%v", outcome, p.Points["1"], err)
	}
	if s.Project.HorizontalCRS == nil || s.Project.HorizontalCRS.Label() != "MGA2020_ZONE50" {
		t.Fatalf("CRS=%+v", s.Project.HorizontalCRS)
	}
	if !strings.Contains(string(s.Project.History[0].Extra), `"elevation":"unchanged"`) {
		t.Fatalf("history=%+v", s.Project.History[0])
	}
	if _, err := s.Execute("undo"); err != nil || len(s.Project.Points) != 0 || s.Project.HorizontalCRS != nil {
		t.Fatalf("undo project=%+v err=%v", s.Project, err)
	}
}

func TestSessionCommitConvertedPointsRequiresCompatibleProjectCoordinates(t *testing.T) {
	p := project.New("converted")
	p.Points["existing"] = geom.Point{ID: "existing"}
	s := NewSession(p, "", "v1")
	request := ConversionCommit{
		Points:       []geom.Point{{ID: "new", Easting: 500000, Northing: 6500000}},
		TargetCRS:    project.HorizontalCRS{Datum: "GDA2020", Projection: "MGA", Zone: 50},
		SourceSystem: "MGA94", TargetSystem: "MGA2020", Model: "conformal",
	}
	if _, err := s.CommitConvertedPoints(request); err == nil || !strings.Contains(err.Error(), "confirm") {
		t.Fatalf("expected CRS confirmation error, got %v", err)
	}
	request.AssumeExisting = true
	if _, err := s.CommitConvertedPoints(request); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitConvertedPoints(ConversionCommit{
		Points: []geom.Point{{ID: "other"}}, TargetCRS: project.HorizontalCRS{Datum: "GDA94", Projection: "MGA", Zone: 50},
	}); err == nil {
		t.Fatal("expected mismatched CRS rejection")
	}
	s.Project.GridGround = &project.GridGroundConversion{Mode: "local_ground"}
	if _, err := s.CommitConvertedPoints(ConversionCommit{
		Points: []geom.Point{{ID: "scaled"}}, TargetCRS: *s.Project.HorizontalCRS,
	}); err == nil {
		t.Fatal("expected active scale rejection")
	}
}

func TestSessionUndoRestoresTraverseScaleAndContourState(t *testing.T) {
	t.Run("traverse", func(t *testing.T) {
		p := project.New("traverse")
		p.Points["1"] = geom.Point{ID: "1"}
		s := NewSession(p, "", "v1")
		if _, err := s.Execute("trav start 1"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Execute("undo"); err != nil {
			t.Fatal(err)
		}
		if s.Project.Traverse != nil {
			t.Fatalf("traverse survived undo: %+v", s.Project.Traverse)
		}
	})

	t.Run("scale", func(t *testing.T) {
		p := project.New("scale")
		p.Points["A"] = geom.Point{ID: "A", Easting: 500000, Northing: 6500000}
		p.Points["B"] = geom.Point{ID: "B", Easting: 500100, Northing: 6500000}
		s := NewSession(p, "", "v1")
		if _, err := s.Execute("scale apply A csf=0.5 system=GROUND"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Execute("undo"); err != nil {
			t.Fatal(err)
		}
		if s.Project.GridGround != nil || s.Project.Points["B"].Easting != 500100 {
			t.Fatalf("scale state survived undo: grid=%+v point=%+v", s.Project.GridGround, s.Project.Points["B"])
		}
	})

	t.Run("contour", func(t *testing.T) {
		p := project.New("contour")
		z0, z1 := 0.0, 2.0
		p.Points["1"] = geom.Point{ID: "1", Easting: 0, Northing: 0, Elevation: &z0}
		p.Points["2"] = geom.Point{ID: "2", Easting: 10, Northing: 0, Elevation: &z1}
		p.Points["3"] = geom.Point{ID: "3", Easting: 0, Northing: 10, Elevation: &z1}
		s := NewSession(p, "", "v1")
		if _, err := s.Execute("contour gen C1 1 breaklines=none"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Execute("undo"); err != nil {
			t.Fatal(err)
		}
		if len(s.Project.ContourSets) != 0 {
			t.Fatalf("contours survived undo: %+v", s.Project.ContourSets)
		}
	})
}

func TestSessionInfoAndHistoryReportsAreReadOnly(t *testing.T) {
	s := NewSession(project.New("job"), "", "v1")
	if _, err := s.Execute("desc Boundary survey"); err != nil {
		t.Fatal(err)
	}
	info, err := s.Execute("info")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"project=job", "path=unsaved", "state=dirty", "history=1", "undo=true", "description=Boundary survey"} {
		if !strings.Contains(info.Message, want) {
			t.Fatalf("info=%q missing %q", info.Message, want)
		}
	}
	list, err := s.Execute("history limit=1")
	if err != nil || !strings.Contains(list.Message, "1 history entries") || !strings.Contains(list.Message, "desc Boundary survey") {
		t.Fatalf("history=%q err=%v", list.Message, err)
	}
	detail, err := s.Execute("history info 1")
	if err != nil || !strings.Contains(detail.Message, "command=desc Boundary survey") {
		t.Fatalf("detail=%q err=%v", detail.Message, err)
	}
	if len(s.Project.History) != 1 {
		t.Fatalf("reports changed history=%d", len(s.Project.History))
	}
	if _, err := s.Execute("history limit=0"); err == nil {
		t.Fatal("expected invalid history limit error")
	}
}
