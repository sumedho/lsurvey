package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"lsurvey/internal/geom"
	"lsurvey/internal/project"
)

type SortField string

const (
	SortID    SortField = "id"
	SortNorth SortField = "north"
	SortEast  SortField = "east"
	SortElev  SortField = "elev"
	SortCode  SortField = "code"
	SortDesc  SortField = "desc"
)

var sortFields = []SortField{SortID, SortEast, SortNorth, SortElev, SortCode, SortDesc}

func ParseSortField(value string) (SortField, bool) {
	switch SortField(strings.ToLower(value)) {
	case SortID, SortNorth, SortEast, SortElev, SortCode, SortDesc:
		return SortField(strings.ToLower(value)), true
	default:
		return "", false
	}
}

func FilterAndSortPoints(p *project.Project, filter string, field SortField, asc bool) []geom.Point {
	filter = strings.ToLower(strings.TrimSpace(filter))
	points := make([]geom.Point, 0, len(p.Points))
	for _, pt := range p.Points {
		if filter == "" || strings.Contains(strings.ToLower(pt.ID), filter) || strings.Contains(strings.ToLower(pt.Code), filter) || strings.Contains(strings.ToLower(pt.Description), filter) {
			points = append(points, pt)
		}
	}
	sort.SliceStable(points, func(i, j int) bool {
		less := pointLess(points[i], points[j], field)
		if asc {
			return less
		}
		return !less && points[i].ID != points[j].ID
	})
	return points
}

func pointLess(a, b geom.Point, field SortField) bool {
	switch field {
	case SortNorth:
		return a.Northing < b.Northing || a.Northing == b.Northing && project.PointIDLess(a.ID, b.ID)
	case SortEast:
		return a.Easting < b.Easting || a.Easting == b.Easting && project.PointIDLess(a.ID, b.ID)
	case SortElev:
		az, bz := elevationValue(a), elevationValue(b)
		return az < bz || az == bz && project.PointIDLess(a.ID, b.ID)
	case SortCode:
		return a.Code < b.Code || a.Code == b.Code && project.PointIDLess(a.ID, b.ID)
	case SortDesc:
		return a.Description < b.Description || a.Description == b.Description && project.PointIDLess(a.ID, b.ID)
	default:
		return project.PointIDLess(a.ID, b.ID)
	}
}

func elevationValue(p geom.Point) float64 {
	if p.Elevation == nil {
		return -1e308
	}
	return *p.Elevation
}

func FormatPointRows(points []geom.Point, groups map[string]project.Group, precision int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-10s %14s %14s %12s %-10s %-10s %-14s %s\n", "ID", "Easting", "Northing", "Elevation", "Code", "Group", "Layer", "Description"))
	b.WriteString(strings.Repeat("-", 106))
	b.WriteByte('\n')
	for _, pt := range points {
		elev := ""
		if pt.Elevation != nil {
			elev = formatDecimal(*pt.Elevation, precision)
		}
		fmt.Fprintf(&b, "%-10s %14s %14s %12s %-10s %-10s %-14s %s\n", pt.ID, formatDecimal(pt.Easting, precision), formatDecimal(pt.Northing, precision), elev, pt.Code, pt.GroupID, groupLayer(groups, pt.GroupID), pt.Description)
	}
	return b.String()
}

func formatDecimal(value float64, precision int) string {
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func FormatLineRows(features []project.Feature, contours []project.ContourSet, groups map[string]project.Group, precision int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-10s %-10s %-24s %-12s %-10s %-14s %s\n", "ID", "Type", "Points", "Code", "Group", "Layer", "Description"))
	b.WriteString(strings.Repeat("-", 100))
	b.WriteByte('\n')
	for _, feature := range features {
		fmt.Fprintf(&b, "%-10s %-10s %-24s %-12s %-10s %-14s %s\n", feature.ID, feature.Kind, strings.Join(feature.PointIDs, ","), feature.Code, feature.GroupID, groupLayer(groups, feature.GroupID), feature.Description)
	}
	if len(contours) > 0 {
		b.WriteString("Contours:\n")
		for _, set := range contours {
			fmt.Fprintf(&b, "  %-10s interval=%s polylines=%d\n", set.ID, formatDecimal(set.Interval, precision), len(set.Polylines))
		}
	}
	return b.String()
}

func groupLayer(groups map[string]project.Group, groupID string) string {
	if groupID == "" {
		return ""
	}
	group, ok := groups[groupID]
	if !ok {
		return ""
	}
	return group.Layer
}
