package cogo

import (
	"fmt"
	"strconv"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

func parseAngleTokens(tokens []string) (geom.Angle, int, error) {
	if len(tokens) >= 3 {
		if a, used, err := geom.ParseQuadrantBearing(tokens[:3]); err == nil {
			return a, used, nil
		}
	}
	if len(tokens) == 0 {
		return geom.Angle{}, 0, fmt.Errorf("missing angle")
	}
	a, err := geom.ParseAngle(tokens[0])
	return a, 1, err
}

func point(p *project.Project, id string) (geom.Point, error) {
	pt, ok := p.Points[id]
	if !ok {
		return geom.Point{}, fmt.Errorf("point %q not found", id)
	}
	return pt, nil
}

func storeCreatedPoint(p *project.Project, pt geom.Point) error {
	if !geom.FinitePoint(pt) {
		return fmt.Errorf("point coordinates must be finite")
	}
	if _, exists := p.Points[pt.ID]; exists {
		return fmt.Errorf("point %q already exists", pt.ID)
	}
	p.Points[pt.ID] = p.ApplyPointCodeStyle(pt)
	return nil
}

func parseFloat(name, value string) (float64, error) {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil || !geom.Finite(f) {
		return 0, fmt.Errorf("invalid %s %q", name, value)
	}
	return f, nil
}

func formatDistance(value float64, precision int) string {
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func optional(fields []string, i int) string {
	if i < len(fields) {
		return fields[i]
	}
	return ""
}
