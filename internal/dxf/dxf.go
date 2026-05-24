package dxf

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

const (
	lineColor          = 6
	lineLabelColor     = 3
	minorContourColor  = 8
	indexContourColor  = 1
	minorContourWeight = 5
	indexContourWeight = 13
	minorContourWidth  = 0.0
	indexContourWidth  = 0.1
	contourLabelHeight = 1.0
	lineLabelHeight    = 1.0
	lineLabelOffset    = 1.0
)

func Write(w io.Writer, p *project.Project) error {
	bw := &writer{w: w}
	bw.pair(0, "SECTION")
	bw.pair(2, "HEADER")
	bw.pair(9, "$ACADVER")
	bw.pair(1, "AC1021")
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
		inv := geom.Inverse(from, to)
		if labels, ok := lineAnnotationLabels(from, to, z1, z2, inv, p.DisplayPrecision()); ok {
			for _, label := range labels {
				bw.centeredMText("LINE_LABELS", lineLabelColor, label)
			}
		}
	}

	var acceptedLabels []contourLabelPlacement
	for _, set := range p.SortedContourSets() {
		polylines := append([]project.ContourPolyline(nil), set.Polylines...)
		sort.SliceStable(polylines, func(i, j int) bool {
			if polylines[i].ID != polylines[j].ID {
				return polylines[i].ID < polylines[j].ID
			}
			return polylines[i].Elevation < polylines[j].Elevation
		})
		for _, contour := range polylines {
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
			if label, required, ok := contourTextLabel(contour); ok && acceptContourLabel(label, required, acceptedLabels) {
				bw.centeredText(labelLayer, color, label)
				acceptedLabels = append(acceptedLabels, contourLabelPlacement{Label: label, RequiredLength: required})
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
		{name: "LINE_LABELS", color: lineLabelColor, lineWeight: 0},
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

type textLabel struct {
	Easting   float64
	Northing  float64
	Elevation float64
	Height    float64
	Rotation  float64
	Value     string
}

func (w *writer) entity(kind, layer string) {
	w.pair(0, kind)
	w.pair(8, layer)
}

func (w *writer) entityColor(kind, layer string, color int) {
	w.entity(kind, layer)
	w.pair(62, color)
}

func (w *writer) centeredText(layer string, color int, label textLabel) {
	w.entityColor("TEXT", layer, color)
	w.pair(10, label.Easting)
	w.pair(20, label.Northing)
	w.pair(30, label.Elevation)
	w.pair(40, label.Height)
	w.pair(1, label.Value)
	w.pair(50, label.Rotation)
	w.pair(72, 1)
	w.pair(73, 2)
	w.pair(11, label.Easting)
	w.pair(21, label.Northing)
	w.pair(31, label.Elevation)
}

func (w *writer) centeredMText(layer string, color int, label textLabel) {
	w.entityColor("MTEXT", layer, color)
	w.pair(10, label.Easting)
	w.pair(20, label.Northing)
	w.pair(30, label.Elevation)
	w.pair(40, label.Height)
	w.pair(41, math.Max(label.Height, float64(len([]rune(label.Value)))*label.Height))
	w.pair(71, 5)
	w.pair(72, 1)
	w.pair(1, encodeDXFText(label.Value))
	rotationRad := label.Rotation * math.Pi / 180
	w.pair(11, math.Cos(rotationRad))
	w.pair(21, math.Sin(rotationRad))
	w.pair(31, 0)
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

type contourLabelPlacement struct {
	Label          textLabel
	RequiredLength float64
}

func contourTextLabel(contour project.ContourPolyline) (textLabel, float64, bool) {
	value := contourElevationLabel(contour.Elevation)
	required := contourLabelHeight * float64(len([]rune(value))+2) * 0.7
	if len(contour.Vertices) < 2 {
		return textLabel{}, required, false
	}
	total := 0.0
	for i := 1; i < len(contour.Vertices); i++ {
		total += contourVertexDistance(contour.Vertices[i-1], contour.Vertices[i])
	}
	if total < required {
		return textLabel{}, required, false
	}
	target := total / 2
	traversed := 0.0
	for i := 1; i < len(contour.Vertices); i++ {
		a, b := contour.Vertices[i-1], contour.Vertices[i]
		length := contourVertexDistance(a, b)
		if length == 0 {
			continue
		}
		if traversed+length+1e-9 < target {
			traversed += length
			continue
		}
		t := (target - traversed) / length
		rotation := math.Atan2(b.Northing-a.Northing, b.Easting-a.Easting) * 180 / math.Pi
		for rotation > 90 {
			rotation -= 180
		}
		for rotation < -90 {
			rotation += 180
		}
		return textLabel{
			Easting:   a.Easting + t*(b.Easting-a.Easting),
			Northing:  a.Northing + t*(b.Northing-a.Northing),
			Elevation: contour.Elevation,
			Height:    contourLabelHeight,
			Rotation:  rotation,
			Value:     value,
		}, required, true
	}
	return textLabel{}, required, false
}

func acceptContourLabel(candidate textLabel, required float64, accepted []contourLabelPlacement) bool {
	for _, prior := range accepted {
		distance := math.Hypot(candidate.Easting-prior.Label.Easting, candidate.Northing-prior.Label.Northing)
		if distance < (required+prior.RequiredLength)/2 {
			return false
		}
	}
	return true
}

func contourVertexDistance(a, b project.ContourVertex) float64 {
	return math.Hypot(a.Easting-b.Easting, a.Northing-b.Northing)
}

func encodeDXFText(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch r {
		case '′':
			b.WriteByte('\'')
			continue
		case '″':
			b.WriteByte('"')
			continue
		}
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
			continue
		}
		fmt.Fprintf(&b, "\\U+%04X", r)
	}
	return b.String()
}

func lineAnnotationLabels(from, to geom.Point, z1, z2 float64, inv geom.InverseResult, precision int) ([]textLabel, bool) {
	if inv.HorizontalDistance == 0 {
		return nil, false
	}
	midE := (from.Easting + to.Easting) / 2
	midN := (from.Northing + to.Northing) / 2
	midZ := (z1 + z2) / 2

	perpRad := inv.Azimuth.Add(geom.AngleFromDegrees(90)).Radians()
	offsetE := math.Sin(perpRad) * lineLabelOffset
	offsetN := math.Cos(perpRad) * lineLabelOffset
	rotation := readableTextRotation(inv.Azimuth.Degrees())

	return []textLabel{
		{
			Easting:   midE + offsetE,
			Northing:  midN + offsetN,
			Elevation: midZ,
			Height:    lineLabelHeight,
			Rotation:  rotation,
			Value:     strconv.FormatFloat(inv.HorizontalDistance, 'f', precision, 64),
		},
		{
			Easting:   midE - offsetE,
			Northing:  midN - offsetN,
			Elevation: midZ,
			Height:    lineLabelHeight,
			Rotation:  rotation,
			Value:     inv.Azimuth.FormatDMS(2),
		},
	}, true
}

func readableTextRotation(angleDeg float64) float64 {
	rotation := math.Mod(90-angleDeg, 360)
	if rotation < 0 {
		rotation += 360
	}
	if rotation > 90 && rotation < 270 {
		rotation = math.Mod(rotation+180, 360)
	}
	return rotation
}
