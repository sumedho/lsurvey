package paths

import "testing"

func TestProjectAppendsSRV(t *testing.T) {
	tests := map[string]string{
		"job":     "job.srv",
		"job.srv": "job.srv",
		"JOB.SRV": "JOB.SRV",
		"":        "",
	}
	for in, want := range tests {
		if got := Project(in); got != want {
			t.Fatalf("Project(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDXFAppendsDXF(t *testing.T) {
	tests := map[string]string{
		"job":     "job.dxf",
		"job.dxf": "job.dxf",
		"JOB.DXF": "JOB.DXF",
		"":        "",
	}
	for in, want := range tests {
		if got := DXF(in); got != want {
			t.Fatalf("DXF(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCSVAppendsCSV(t *testing.T) {
	tests := map[string]string{
		"points":     "points.csv",
		"points.csv": "points.csv",
		"POINTS.CSV": "POINTS.CSV",
		"":           "",
	}
	for in, want := range tests {
		if got := CSV(in); got != want {
			t.Fatalf("CSV(%q)=%q want %q", in, got, want)
		}
	}
}

func TestGeoJSONAppendsGeoJSON(t *testing.T) {
	tests := map[string]string{
		"points":         "points.geojson",
		"points.geojson": "points.geojson",
		"POINTS.GEOJSON": "POINTS.GEOJSON",
		"":               "",
	}
	for in, want := range tests {
		if got := GeoJSON(in); got != want {
			t.Fatalf("GeoJSON(%q)=%q want %q", in, got, want)
		}
	}
}
