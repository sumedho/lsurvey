package boundary

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

var CSVHeader = []string{"polygon_id", "polygon_code", "polygon_description", "area", "perimeter", "leg", "from", "to", "bearing", "distance"}

type Leg struct {
	Number   int
	From     string
	To       string
	Bearing  geom.Angle
	Distance float64
}

type Schedule struct {
	Feature   project.Feature
	Area      float64
	Perimeter float64
	Legs      []Leg
}

func ForPolygon(p *project.Project, id string) (Schedule, error) {
	feature, ok := p.Features[id]
	if !ok || feature.Kind != project.FeaturePolygon {
		return Schedule{}, fmt.Errorf("polygon %q not found", id)
	}
	return forFeature(p, feature), nil
}

func ForFeature(p *project.Project, feature project.Feature) (Schedule, bool) {
	if feature.Kind != project.FeaturePolygon || len(feature.PointIDs) < 3 {
		return Schedule{}, false
	}
	return forFeature(p, feature), true
}

func All(p *project.Project) []Schedule {
	var schedules []Schedule
	for _, feature := range p.SortedFeatures() {
		if feature.Kind == project.FeaturePolygon {
			schedules = append(schedules, forFeature(p, feature))
		}
	}
	return schedules
}

func forFeature(p *project.Project, feature project.Feature) Schedule {
	points := make([]geom.Point, 0, len(feature.PointIDs))
	for _, id := range feature.PointIDs {
		points = append(points, p.Points[id])
	}
	closeResult, _ := geom.Close(points)
	schedule := Schedule{Feature: feature, Area: closeResult.Area, Perimeter: closeResult.Perimeter}
	for i, segment := range p.FeatureSegments(feature) {
		inv := geom.Inverse(p.Points[segment.From], p.Points[segment.To])
		schedule.Legs = append(schedule.Legs, Leg{Number: i + 1, From: segment.From, To: segment.To, Bearing: inv.Azimuth, Distance: inv.HorizontalDistance})
	}
	return schedule
}

func ExportFile(path string, p *project.Project, polygonID string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = WriteCSV(f, p, polygonID)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func WriteCSV(w io.Writer, p *project.Project, polygonID string) error {
	schedules := All(p)
	if polygonID != "" {
		schedule, err := ForPolygon(p, polygonID)
		if err != nil {
			return err
		}
		schedules = []Schedule{schedule}
	}
	writer := csv.NewWriter(w)
	if err := writer.Write(CSVHeader); err != nil {
		return err
	}
	precision := p.DisplayPrecision()
	for _, schedule := range schedules {
		for _, leg := range schedule.Legs {
			row := []string{
				schedule.Feature.ID, schedule.Feature.Code, schedule.Feature.Description,
				format(schedule.Area, precision), format(schedule.Perimeter, precision),
				strconv.Itoa(leg.Number), leg.From, leg.To, leg.Bearing.FormatDMS(2), format(leg.Distance, precision),
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
	}
	writer.Flush()
	return writer.Error()
}

func LabelPoint(p *project.Project, feature project.Feature) geom.Point {
	points := polygonPoints(p, feature)
	if centroid, ok := polygonCentroid(points); ok && ContainsPoint(p, feature, centroid) {
		return centroid
	}
	var ys []float64
	for _, pt := range points {
		ys = append(ys, pt.Northing)
	}
	sort.Float64s(ys)
	bestWidth := -1.0
	best := points[0]
	for i := 1; i < len(ys); i++ {
		if ys[i] == ys[i-1] {
			continue
		}
		y := (ys[i] + ys[i-1]) / 2
		var xs []float64
		for j, a := range points {
			b := points[(j+1)%len(points)]
			if (a.Northing <= y && y < b.Northing) || (b.Northing <= y && y < a.Northing) {
				t := (y - a.Northing) / (b.Northing - a.Northing)
				xs = append(xs, a.Easting+t*(b.Easting-a.Easting))
			}
		}
		sort.Float64s(xs)
		for j := 0; j+1 < len(xs); j += 2 {
			if width := xs[j+1] - xs[j]; width > bestWidth {
				bestWidth = width
				best = geom.Point{Easting: (xs[j] + xs[j+1]) / 2, Northing: y}
			}
		}
	}
	return best
}

func ContainsPoint(p *project.Project, feature project.Feature, point geom.Point) bool {
	points := polygonPoints(p, feature)
	inside := false
	for i, a := range points {
		b := points[(i+1)%len(points)]
		if (a.Northing > point.Northing) != (b.Northing > point.Northing) &&
			point.Easting < (b.Easting-a.Easting)*(point.Northing-a.Northing)/(b.Northing-a.Northing)+a.Easting {
			inside = !inside
		}
	}
	return inside
}

func polygonPoints(p *project.Project, feature project.Feature) []geom.Point {
	points := make([]geom.Point, 0, len(feature.PointIDs))
	for _, id := range feature.PointIDs {
		points = append(points, p.Points[id])
	}
	return points
}

func polygonCentroid(points []geom.Point) (geom.Point, bool) {
	var twiceArea, east, north float64
	for i, a := range points {
		b := points[(i+1)%len(points)]
		cross := a.Easting*b.Northing - b.Easting*a.Northing
		twiceArea += cross
		east += (a.Easting + b.Easting) * cross
		north += (a.Northing + b.Northing) * cross
	}
	if math.Abs(twiceArea) < 1e-12 {
		return geom.Point{}, false
	}
	return geom.Point{Easting: east / (3 * twiceArea), Northing: north / (3 * twiceArea)}, true
}

func format(value float64, precision int) string {
	return strconv.FormatFloat(value, 'f', precision, 64)
}
