package dxf

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"lsurvey/internal/project"
)

const (
	minorContourColor  = 8
	indexContourColor  = 1
	contourLabelHeight = 1.0
)

func Write(w io.Writer, p *project.Project) error {
	bw := &writer{w: w}
	bw.pair(0, "SECTION")
	bw.pair(2, "HEADER")
	bw.pair(9, "$ACADVER")
	bw.pair(1, "AC1009")
	bw.pair(0, "ENDSEC")
	bw.pair(0, "SECTION")
	bw.pair(2, "ENTITIES")

	for _, pt := range p.SortedPoints() {
		z := 0.0
		if pt.Elevation != nil {
			z = *pt.Elevation
		}
		bw.entity("POINT", layer("POINTS", pt.Code))
		bw.pair(10, pt.Easting)
		bw.pair(20, pt.Northing)
		bw.pair(30, z)

		label := pt.ID
		if pt.Code != "" {
			label += " " + pt.Code
		}
		bw.entity("TEXT", layer("LABELS", pt.Code))
		bw.pair(10, pt.Easting)
		bw.pair(20, pt.Northing)
		bw.pair(30, z)
		bw.pair(40, 1.0)
		bw.pair(1, label)
	}

	for _, line := range p.SortedLines() {
		from, ok1 := p.Points[line.From]
		to, ok2 := p.Points[line.To]
		if !ok1 || !ok2 {
			continue
		}
		z1, z2 := 0.0, 0.0
		if from.Elevation != nil {
			z1 = *from.Elevation
		}
		if to.Elevation != nil {
			z2 = *to.Elevation
		}
		bw.entity("LINE", layer("LINES", line.Code))
		bw.pair(10, from.Easting)
		bw.pair(20, from.Northing)
		bw.pair(30, z1)
		bw.pair(11, to.Easting)
		bw.pair(21, to.Northing)
		bw.pair(31, z2)
	}

	for _, set := range p.SortedContourSets() {
		for _, contour := range set.Polylines {
			layerName := "CONTOURS"
			labelLayer := "CONTOUR_LABELS"
			color := minorContourColor
			if contour.Index {
				layerName = "CONTOURS_INDEX"
				labelLayer = "CONTOUR_LABELS_INDEX"
				color = indexContourColor
			}
			if len(contour.Vertices) < 2 {
				continue
			}
			bw.entityColor("POLYLINE", layerName, color)
			bw.pair(66, 1)
			bw.pair(70, 0)
			bw.pair(30, contour.Elevation)
			for _, v := range contour.Vertices {
				bw.entityColor("VERTEX", layerName, color)
				bw.pair(10, v.Easting)
				bw.pair(20, v.Northing)
				bw.pair(30, contour.Elevation)
			}
			bw.pair(0, "SEQEND")
			if label, ok := contourLabelPoint(contour.Vertices); ok {
				bw.entityColor("TEXT", labelLayer, color)
				bw.pair(10, label.Easting)
				bw.pair(20, label.Northing)
				bw.pair(30, contour.Elevation)
				bw.pair(40, contourLabelHeight)
				bw.pair(1, contourElevationLabel(contour.Elevation))
			}
		}
	}

	bw.pair(0, "ENDSEC")
	bw.pair(0, "EOF")
	return bw.err
}

type writer struct {
	w   io.Writer
	err error
}

func (w *writer) entity(kind, layer string) {
	w.pair(0, kind)
	w.pair(8, layer)
}

func (w *writer) entityColor(kind, layer string, color int) {
	w.entity(kind, layer)
	w.pair(62, color)
}

func (w *writer) pair(code int, value any) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintf(w.w, "%d\n%v\n", code, value)
}

func layer(prefix, code string) string {
	if code == "" {
		return prefix
	}
	code = strings.ToUpper(code)
	code = strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, code)
	return prefix + "_" + code
}

func contourElevationLabel(elevation float64) string {
	return strconv.FormatFloat(elevation, 'f', -1, 64)
}

func contourLabelPoint(vertices []project.ContourVertex) (project.ContourVertex, bool) {
	if len(vertices) < 2 {
		return project.ContourVertex{}, false
	}
	total := 0.0
	for i := 1; i < len(vertices); i++ {
		total += vertexDistance(vertices[i-1], vertices[i])
	}
	if total == 0 {
		return vertices[0], true
	}
	target := total / 2
	walked := 0.0
	for i := 1; i < len(vertices); i++ {
		a := vertices[i-1]
		b := vertices[i]
		d := vertexDistance(a, b)
		if d == 0 {
			continue
		}
		if walked+d >= target {
			t := (target - walked) / d
			return project.ContourVertex{
				Northing: a.Northing + t*(b.Northing-a.Northing),
				Easting:  a.Easting + t*(b.Easting-a.Easting),
			}, true
		}
		walked += d
	}
	return vertices[len(vertices)-1], true
}

func vertexDistance(a, b project.ContourVertex) float64 {
	return math.Hypot(b.Easting-a.Easting, b.Northing-a.Northing)
}
