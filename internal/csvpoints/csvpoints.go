package csvpoints

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

var Header = []string{"id", "easting", "northing", "elevation", "code", "description"}

func ImportFile(path string, p *project.Project) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return Import(f, p)
}

func ExportFile(path string, p *project.Project) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = Export(f, p)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func Import(r io.Reader, p *project.Project) (int, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = len(Header)
	header, err := reader.Read()
	if err != nil {
		return 0, err
	}
	if !sameHeader(header) {
		return 0, fmt.Errorf("invalid CSV header: expected %q", Header)
	}
	points := make(map[string]geom.Point)
	count := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			for id, pt := range points {
				p.Points[id] = pt
			}
			if len(points) > 0 {
				p.MarkContoursStale("imported point geometry changed")
			}
			return count, nil
		}
		if err != nil {
			return count, err
		}
		pt, err := pointFromRow(row)
		if err != nil {
			return count, fmt.Errorf("row %d: %w", count+2, err)
		}
		pt = p.ApplyPointCodeStyle(pt)
		points[pt.ID] = pt
		count++
	}
}

func Export(w io.Writer, p *project.Project) error {
	writer := csv.NewWriter(w)
	if err := writer.Write(Header); err != nil {
		return err
	}
	for _, pt := range p.SortedPoints() {
		elev := ""
		if pt.Elevation != nil {
			elev = strconv.FormatFloat(*pt.Elevation, 'f', -1, 64)
		}
		if err := writer.Write([]string{
			pt.ID,
			strconv.FormatFloat(pt.Easting, 'f', -1, 64),
			strconv.FormatFloat(pt.Northing, 'f', -1, 64),
			elev,
			pt.Code,
			pt.Description,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func pointFromRow(row []string) (geom.Point, error) {
	if row[0] == "" {
		return geom.Point{}, fmt.Errorf("id is required")
	}
	easting, err := strconv.ParseFloat(row[1], 64)
	if err != nil {
		return geom.Point{}, fmt.Errorf("invalid easting %q", row[1])
	}
	northing, err := strconv.ParseFloat(row[2], 64)
	if err != nil {
		return geom.Point{}, fmt.Errorf("invalid northing %q", row[2])
	}
	var elev *float64
	if row[3] != "" {
		value, err := strconv.ParseFloat(row[3], 64)
		if err != nil {
			return geom.Point{}, fmt.Errorf("invalid elevation %q", row[3])
		}
		elev = &value
	}
	return geom.Point{
		ID:          row[0],
		Easting:     easting,
		Northing:    northing,
		Elevation:   elev,
		Code:        row[4],
		Description: row[5],
	}, nil
}

func sameHeader(got []string) bool {
	if len(got) != len(Header) {
		return false
	}
	for i := range Header {
		if got[i] != Header[i] {
			return false
		}
	}
	return true
}
