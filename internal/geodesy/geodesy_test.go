package geodesy

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestKruegerProjectionRoundTripAndCentralMeridian(t *testing.T) {
	input := Geographic{Datum: GDA2020, Latitude: -32, Longitude: 117}
	grid, err := GeographicToGrid(input, 50)
	if err != nil {
		t.Fatal(err)
	}
	closeValue(t, grid.Easting, 500000, 1e-7)
	got, err := GridToGeographic(grid)
	if err != nil {
		t.Fatal(err)
	}
	closeValue(t, got.Latitude, input.Latitude, 1e-10)
	closeValue(t, got.Longitude, input.Longitude, 1e-10)
}

func TestKruegerProjectionMatchesPerthMGAReferenceCoordinates(t *testing.T) {
	for _, test := range []struct {
		latitude, longitude string
		easting, northing   float64
	}{
		{latitude: "-31.5710", longitude: "115.5110", easting: 391580.017, northing: 6464224.032},
		{latitude: "-31.5701", longitude: "115.5101", easting: 391340.794, northing: 6464498.654},
	} {
		latitude, err := ParseLatitude(test.latitude)
		if err != nil {
			t.Fatal(err)
		}
		longitude, err := ParseLongitude(test.longitude)
		if err != nil {
			t.Fatal(err)
		}
		grid, err := GeographicToGrid(Geographic{Datum: GDA2020, Latitude: latitude, Longitude: longitude}, 50)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(grid.Easting-test.easting) > 0.001 || math.Abs(grid.Northing-test.northing) > 0.001 {
			t.Fatalf("%s,%s -> grid=(%.3f E, %.3f N) want (%.3f E, %.3f N)",
				test.latitude, test.longitude, grid.Easting, grid.Northing, test.easting, test.northing)
		}
	}
}

func TestMGAZoneForLongitudeDerivesPerthZone(t *testing.T) {
	zone, err := MGAZoneForLongitude(115 + 51.0/60 + 1.0/3600)
	if err != nil || zone != 50 {
		t.Fatalf("zone=%d err=%v want 50", zone, err)
	}
}

func TestConvertProjectsWithinDatumAndRequiresTransformBetweenDatums(t *testing.T) {
	input := Coordinate{Geographic: &Geographic{Datum: GDA2020, Latitude: -32, Longitude: 117}}
	got, err := Convert(Request{Source: input, Target: MGA2020System, TargetZone: 50})
	if err != nil || got.Grid == nil || got.Grid.Datum != GDA2020 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if _, err := Convert(Request{Source: input, Target: MGA94System, TargetZone: 50}); err == nil {
		t.Fatal("expected cross-datum conversion to require a transformer")
	}
}

func TestNTv2InterpolatesForwardAndReverseWithoutChangingElevation(t *testing.T) {
	grid, err := ReadNTv2(miniGrid(t))
	if err != nil {
		t.Fatal(err)
	}
	z := 12.5
	source := Geographic{Datum: GDA94, Latitude: -31.5, Longitude: 116.5, Elevation: &z}
	got, err := grid.Forward(source)
	if err != nil {
		t.Fatal(err)
	}
	closeValue(t, got.Latitude, source.Latitude+1.5/3600, 1e-12)
	closeValue(t, got.Longitude, source.Longitude-2.5/3600, 1e-12)
	if got.Elevation == nil || *got.Elevation != z {
		t.Fatalf("elevation=%v want unchanged", got.Elevation)
	}
	reversed, err := grid.Reverse(got)
	if err != nil {
		t.Fatal(err)
	}
	closeValue(t, reversed.Latitude, source.Latitude, 1e-11)
	closeValue(t, reversed.Longitude, source.Longitude, 1e-11)
}

func TestNTv2RejectsCoordinatesOutsideCoverage(t *testing.T) {
	grid, err := ReadNTv2(miniGrid(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := grid.Forward(Geographic{Datum: GDA94, Latitude: -25, Longitude: 116.5}); err == nil {
		t.Fatal("expected coverage error")
	}
}

func TestLoadNTv2RequiresGridFilenameMatchingSelectedModel(t *testing.T) {
	dir := t.TempDir()
	data, err := io.ReadAll(miniGrid(t))
	if err != nil {
		t.Fatal(err)
	}
	distortionPath := filepath.Join(dir, "GDA94_GDA2020_conformal_and_distortion.gsb")
	if err := os.WriteFile(distortionPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadNTv2(distortionPath, Conformal); err == nil {
		t.Fatal("expected conformal selection to reject distortion grid file")
	}
	if _, err := LoadNTv2(distortionPath, ConformalDistortion); err != nil {
		t.Fatal(err)
	}
}

func miniGrid(t *testing.T) *bytes.Reader {
	t.Helper()
	var data bytes.Buffer
	order := binary.LittleEndian
	writeText := func(label, value string) {
		data.WriteString(padded(label))
		data.WriteString(padded(value))
	}
	writeInt := func(label string, value int32) {
		data.WriteString(padded(label))
		var raw [8]byte
		order.PutUint32(raw[:4], uint32(value))
		data.Write(raw[:])
	}
	writeFloat := func(label string, value float64) {
		data.WriteString(padded(label))
		var raw [8]byte
		order.PutUint64(raw[:], math.Float64bits(value))
		data.Write(raw[:])
	}
	writeInt("NUM_OREC", 11)
	writeInt("NUM_SREC", 11)
	writeInt("NUM_FILE", 1)
	writeText("GS_TYPE", "SECONDS")
	writeText("VERSION", "TEST")
	writeText("SYSTEM_F", "GDA94")
	writeText("SYSTEM_T", "GDA2020")
	writeFloat("MAJOR_F", grs80A)
	writeFloat("MINOR_F", 6356752.3141)
	writeFloat("MAJOR_T", grs80A)
	writeFloat("MINOR_T", 6356752.3141)
	writeText("SUB_NAME", "TEST")
	writeText("PARENT", "NONE")
	writeText("CREATED", "")
	writeText("UPDATED", "")
	writeFloat("S_LAT", -32*3600)
	writeFloat("N_LAT", -31*3600)
	writeFloat("E_LONG", -117*3600)
	writeFloat("W_LONG", -116*3600)
	writeFloat("LAT_INC", 3600)
	writeFloat("LONG_INC", 3600)
	writeInt("GS_COUNT", 4)
	for _, node := range [][2]float32{{1, 2}, {1, 2}, {2, 3}, {2, 3}} {
		if err := binary.Write(&data, order, [4]float32{node[0], node[1], 0, 0}); err != nil {
			t.Fatal(err)
		}
	}
	return bytes.NewReader(data.Bytes())
}

func padded(value string) string {
	for len(value) < 8 {
		value += " "
	}
	return value[:8]
}

func closeValue(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("got %.12f want %.12f", got, want)
	}
}
