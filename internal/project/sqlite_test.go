package project

import (
	"database/sql"
	"encoding/json"
	"lsurvey/internal/geom"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSQLiteNativeTablesAndConstraints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native # ? job.srv")
	p := validIntegrityProject()
	z := 0.0
	p.Points["B"] = geom.Point{ID: "B", Easting: 500000.123456789, Northing: 6500000.987654321, Elevation: &z}
	p.AddHistoryChange("pt add B", "added", []string{"point:B"}, nil, nil, map[string]any{"method": "test", "residuals": []float64{.001, .002}}, nil)
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data[:16]) != "SQLite format 3\x00" {
		t.Fatal("not a native SQLite database")
	}
	db, err := openSQLite(path, "rw")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var e float64
	var elevation sql.NullFloat64
	if err = db.QueryRow("SELECT easting, elevation FROM points WHERE id='B'").Scan(&e, &elevation); err != nil {
		t.Fatal(err)
	}
	if e != p.Points["B"].Easting || !elevation.Valid || elevation.Float64 != 0 {
		t.Fatal("coordinate precision/zero elevation lost")
	}
	if err = db.QueryRow("SELECT elevation FROM points WHERE id='A'").Scan(&elevation); err != nil || elevation.Valid {
		t.Fatal("2D elevation not NULL")
	}
	if _, err = db.Exec("INSERT INTO feature_vertices(feature_id,ordinal,point_id) VALUES ('missing',0,'A')"); err == nil {
		t.Fatal("foreign keys not enforced")
	}
	if _, err = db.Exec("UPDATE audit_events SET command='tampered'"); err == nil {
		t.Fatal("audit is mutable")
	}
	if _, err = db.Exec("DELETE FROM audit_events"); err == nil {
		t.Fatal("audit can be deleted")
	}
}

func TestSQLiteRejectsUnknownVersionAndForeignDatabase(t *testing.T) {
	for _, pragma := range []string{"PRAGMA user_version=999", "PRAGMA application_id=42"} {
		path := filepath.Join(t.TempDir(), "job.srv")
		if err := Save(path, New("test")); err != nil {
			t.Fatal(err)
		}
		db, err := openSQLite(path, "rw")
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(pragma)
		db.Close()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("accepted unsupported database")
		}
		if err := Save(path, New("replacement")); err == nil {
			t.Fatal("overwrote unsupported database")
		}
	}
	missing := filepath.Join(t.TempDir(), "absent.srv")
	if _, err := Load(missing); err == nil {
		t.Fatal("loaded missing file")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("load created database")
	}
}

func TestSQLiteAuditPrefixAndTransactionRollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "job.srv")
	p := validIntegrityProject()
	p.AddHistory("first", "", nil, nil)
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	p.History[0].Command = "rewrite"
	if err := Save(path, p); err == nil {
		t.Fatal("rewrote persisted audit")
	}
	p.History[0].Command = "first"
	p.Name = "edited"
	p.AddHistory("second", "", nil, nil)
	store := SQLiteStore{beforeCommit: func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO feature_vertices VALUES ('missing',0,'A')")
		return err
	}}
	if err := store.Save(path, p); err == nil {
		t.Fatal("expected injected SQL failure")
	}
	got, err := Load(path)
	if err != nil || got.Name != "integrity" || len(got.History) != 1 {
		t.Fatalf("rollback failed: %+v %v", got, err)
	}
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path)
	if err != nil || got.Name != "edited" || len(got.History) != 2 {
		t.Fatal("second save failed")
	}
}

func TestLegacyJSONRequiresSaveAs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.srv")
	data := []byte(`{"schema_version":8,"name":"legacy","points":{},"history":[]}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = Save(path, p); err == nil {
		t.Fatal("overwrote legacy JSON")
	}
	if err = Save(filepath.Join(t.TempDir(), "converted.srv"), p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(data) {
		t.Fatal("legacy changed")
	}
}

func TestSQLiteCompleteProjectRoundTrip(t *testing.T) {
	p := New("complete")
	p.Description = "all persisted domains"
	p.AppVersion = "test"
	p.SetDisplayPrecision(6)
	z, base, maxEdge := 12.3456789, 0.0, 20.0
	p.Groups["G"] = Group{ID: "G", Layer: "Survey", Color: 42, Description: "style"}
	p.PointCodeStyles["PEG"] = "G"
	p.Points = map[string]geom.Point{
		"A": {ID: "A", Easting: 500000, Northing: 6500000, Elevation: &z, Code: "PEG", Description: "control", GroupID: "G"},
		"B": {ID: "B", Easting: 500010, Northing: 6500000, Elevation: &z},
		"C": {ID: "C", Easting: 500000, Northing: 6500010, Elevation: &z},
		"D": {ID: "D", Easting: 500001, Northing: 6500011},
	}
	p.Features["L"] = Feature{ID: "L", Kind: FeatureLine, PointIDs: []string{"A", "B"}, TerrainRole: "ridge", GroupID: "G", Code: "BND", Description: "line"}
	p.Features["PL"] = Feature{ID: "PL", Kind: FeaturePolyline, PointIDs: []string{"B", "C", "A"}}
	p.Features["PG"] = Feature{ID: "PG", Kind: FeaturePolygon, PointIDs: []string{"A", "B", "C"}}
	p.HorizontalCRS = &HorizontalCRS{Datum: "GDA2020", Projection: "MGA", Zone: 50}
	p.GridGround = &GridGroundConversion{Mode: "local_ground", GridSystem: "ground", AnchorPointID: "A", AnchorEasting: 500000, AnchorNorthing: 6500000, CSF: .9996}
	p.Traverse = &TraverseState{Start: "A", Current: "C", Close: "D", LegPointIDs: []string{"B", "C"}, Adjusted: true}
	line := ContourPolyline{ID: "c1", Elevation: 12.5, Index: true, Vertices: []ContourVertex{{Easting: 500001, Northing: 6500002}, {Easting: 500003, Northing: 6500004}}}
	p.ContourSets["C1"] = ContourSet{ID: "C1", Interval: .5, Base: 0, IndexEvery: 2, Stale: true, StaleReason: "deleted source", GeneratedAt: time.Date(2026, 9, 7, 1, 2, 3, 123456789, time.UTC), TriangleCount: 3, EffectiveMaxEdge: 20,
		SourcePoints: []string{"A", "deleted"}, Breaklines: []string{"L"}, BoundaryLines: []string{"old-boundary"}, ExclusionLines: []string{"old-exclusion"}, BreaklineRoles: map[string]string{"L": "ridge"},
		Generation:  &ContourGenerationSpec{Interval: .5, Base: &base, IndexEvery: 2, IndexEverySet: true, BreaklineMode: "ids", BreaklineIDs: []string{"L"}, BoundaryCodes: []string{"BND"}, ExclusionCodes: []string{"VOID"}, MaxEdge: &maxEdge, Smooth: 2},
		Diagnostics: []ContourDiagnostic{{Code: "long_edge", Message: "warning", Measured: 21, Limit: 20, PointIDs: []string{"A", "deleted"}, EdgeIDs: []string{"L"}}}, RawPolylines: []ContourPolyline{line}, Polylines: []ContourPolyline{line},
	}
	p.History = []HistoryRecord{{At: time.Date(2026, 9, 7, 0, 0, 0, 987654321, time.UTC), Command: "test", Result: "result", Created: []string{"point:A"}, Updated: []string{"line:L"}, Deleted: []string{"point:deleted"}, Error: "historic error", Extra: json.RawMessage(`{"large":9007199254740993,"values":[true,false,null,"",{},[],{"x":1.2500}],"scientific":1e-12}`)}}
	path := filepath.Join(t.TempDir(), "complete.srv")
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := decodeDetails(p.History[0].Extra)
	b, _ := decodeDetails(got.History[0].Extra)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("audit detail loss: %s", got.History[0].Extra)
	}
	want := p.Clone()
	want.History[0].Extra = nil
	got.History[0].Extra = nil
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip changed model\nwant=%+v\ngot=%+v", want, got)
	}
}

func TestSQLiteSaveUpdatesInPlaceAndBackupIncludesWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.srv")
	p := validIntegrityProject()
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	db, err := openSQLite(path, "rw")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; UPDATE points SET easting=25 WHERE id='B'"); err != nil {
		t.Fatal(err)
	}
	p, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p.Name = "saved"
	if err = Save(path, p); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("existing database was replaced instead of updated transactionally")
	}
	backup, err := (SQLiteStore{}).Recover(path)
	if err != nil || backup.Points["B"].Easting != 25 || backup.Name != "integrity" {
		t.Fatalf("WAL snapshot lost data: %+v %v", backup, err)
	}
}
