package dxf

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"lsurvey/internal/project"
)

const (
	lineColor          = 6
	minorContourColor  = 8
	indexContourColor  = 1
	minorContourWeight = 5
	indexContourWeight = 13
	minorContourWidth  = 0.0
	indexContourWidth  = 0.1
	contourLabelHeight = 1.0
)

func Write(w io.Writer, p *project.Project) error {
	bw := &writer{w: w}
	bw.pair(0, "SECTION")
	bw.pair(2, "HEADER")
	bw.pair(9, "$ACADVER")
	bw.pair(1, "AC1015")
	bw.pair(0, "ENDSEC")

	bw.pair(0, "SECTION")
	bw.pair(2, "TABLES")
	writeLayerTable(bw)
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
		bw.entityColor("LINE", layer("LINES", line.Code), lineColor)
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
			lineWeight := minorContourWeight
			width := minorContourWidth
			if contour.Index {
				layerName = "CONTOURS_INDEX"
				labelLayer = "CONTOUR_LABELS_INDEX"
				color = indexContourColor
				lineWeight = indexContourWeight
				width = indexContourWidth
			}
			if len(contour.Vertices) < 2 {
				continue
			}
			bw.entityColor("LWPOLYLINE", layerName, color)
			bw.pair(370, lineWeight)
			bw.pair(90, len(contour.Vertices))
			bw.pair(70, 0)
			bw.pair(38, contour.Elevation)
			bw.pair(43, width)
			for _, v := range contour.Vertices {
				bw.pair(10, v.Easting)
				bw.pair(20, v.Northing)
			}
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

func writeLayerTable(w *writer) {
	layers := []layerDef{
		{name: "CONTOURS", color: minorContourColor, lineWeight: minorContourWeight},
		{name: "CONTOURS_INDEX", color: indexContourColor, lineWeight: indexContourWeight},
		{name: "CONTOUR_LABELS", color: minorContourColor, lineWeight: minorContourWeight},
		{name: "CONTOUR_LABELS_INDEX", color: indexContourColor, lineWeight: indexContourWeight},
	}

	w.pair(0, "TABLE")
	w.pair(2, "LAYER")
	w.pair(70, len(layers))
	for _, layer := range layers {
		w.pair(0, "LAYER")
		w.pair(2, layer.name)
		w.pair(70, 0)
		w.pair(62, layer.color)
		w.pair(6, "CONTINUOUS")
		w.pair(370, layer.lineWeight)
	}
	w.pair(0, "ENDTAB")
}

type layerDef struct {
	name       string
	color      int
	lineWeight int
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
	return vertices[len(vertices)-1], true
}
