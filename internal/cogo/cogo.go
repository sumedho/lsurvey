package cogo

import (
	"fmt"
	"strings"

	"lsurvey/internal/project"
)

type Result struct {
	Message string
	Created []string
	Updated []string
	Changed bool
}

func Execute(p *project.Project, command string) (Result, error) {
	fields, err := tokenize(command)
	if err != nil {
		return Result{}, err
	}
	if len(fields) == 0 {
		return Result{}, fmt.Errorf("empty command")
	}
	var result Result
	switch fields[0] {
	case "pt":
		result, err = execPoint(p, fields)
	case "line":
		result, err = execLine(p, fields)
	case "polyline":
		result, err = execOrderedFeature(p, fields, project.FeaturePolyline)
	case "polygon":
		result, err = execOrderedFeature(p, fields, project.FeaturePolygon)
	case "group":
		result, err = execGroup(p, fields)
	case "code":
		result, err = execCode(p, fields)
	case "inverse":
		result, err = execInverse(p, fields)
	case "angle":
		result, err = execAngle(p, fields)
	case "close":
		result, err = execClose(p, fields)
	case "bearing":
		result, err = execBearing(p, fields)
	case "dist":
		result, err = execDistance(p, fields)
	case "rad":
		result, err = execRad(p, fields)
	case "rad3d":
		result, err = execRad3D(p, fields)
	case "midpoint":
		result, err = execMidpoint(p, fields)
	case "offset":
		result, err = execOffset(p, fields)
	case "intersect":
		result, err = execIntersect(p, fields)
	case "resect":
		result, err = execResect(p, fields)
	case "shift":
		result, err = execShift(p, fields)
	case "rotate":
		result, err = execRotate(p, fields)
	case "scale":
		result, err = execScale(p, fields)
	case "transform":
		result, err = execTransform(p, fields)
	case "trav":
		result, err = execTraverse(p, fields)
	case "contour":
		result, err = execContour(p, fields)
	case "units":
		result, err = execUnits(p, fields)
	default:
		return Result{}, fmt.Errorf("unknown command %q", fields[0])
	}
	if err == nil {
		result.Changed = commandChangesProject(fields, result)
		markContoursStaleAfterCommand(p, fields, result)
	}
	return result, err
}

func ExecuteAndRecord(p *project.Project, command string) (Result, error) {
	result, err := Execute(p, command)
	if err == nil && result.Changed {
		p.AddHistoryChange(command, result.Message, result.Created, result.Updated, nil, nil)
	}
	return result, err
}

func commandChangesProject(fields []string, result Result) bool {
	switch fields[0] {
	case "pt":
		return len(fields) > 1 && fields[1] != "list"
	case "line":
		if len(fields) > 1 && fields[1] == "list" {
			return false
		}
		if len(fields) > 1 && fields[1] == "gen" {
			return len(result.Created) > 0
		}
		return true
	case "polyline":
		return len(fields) > 1 && fields[1] != "list" && fields[1] != "info"
	case "polygon":
		return len(fields) > 1 && fields[1] != "list" && fields[1] != "info" && fields[1] != "report"
	case "group":
		return len(fields) > 1 && fields[1] != "list" && fields[1] != "info"
	case "code":
		return len(fields) > 2 && fields[2] != "list"
	case "rad", "rad3d", "midpoint", "offset", "intersect", "resect",
		"shift", "rotate", "scale", "units":
		return true
	case "trav":
		return len(fields) > 1 && fields[1] != "show"
	case "contour":
		return len(fields) > 1 && fields[1] != "list" && fields[1] != "info"
	default:
		return false
	}
}

func markContoursStaleAfterCommand(p *project.Project, fields []string, result Result) {
	if len(fields) == 0 || len(p.ContourSets) == 0 {
		return
	}
	reason := ""
	switch fields[0] {
	case "pt":
		switch fields[1] {
		case "add", "del", "rename":
			reason = "point geometry changed"
		case "edit":
			for _, arg := range fields[3:] {
				if strings.HasPrefix(arg, "east=") || strings.HasPrefix(arg, "easting=") ||
					strings.HasPrefix(arg, "e=") || strings.HasPrefix(arg, "north=") ||
					strings.HasPrefix(arg, "northing=") || strings.HasPrefix(arg, "n=") ||
					strings.HasPrefix(arg, "elev=") || strings.HasPrefix(arg, "z=") {
					reason = "point geometry changed"
				}
			}
		}
	case "line", "polyline", "polygon":
		switch fields[1] {
		case "add", "del":
			reason = "feature geometry changed"
		case "gen":
			if len(result.Created) > 0 {
				reason = "line geometry changed"
			}
		case "edit":
			for _, arg := range fields[3:] {
				if strings.HasPrefix(arg, "from=") || strings.HasPrefix(arg, "to=") ||
					strings.HasPrefix(arg, "points=") || strings.HasPrefix(arg, "code=") ||
					strings.HasPrefix(arg, "terrain=") {
					reason = "feature terrain input changed"
				}
			}
		}
	default:
		for _, item := range append(append([]string(nil), result.Created...), result.Updated...) {
			if strings.HasPrefix(item, "point:") || strings.HasPrefix(item, "line:") ||
				strings.HasPrefix(item, "polyline:") || strings.HasPrefix(item, "polygon:") {
				reason = "point geometry changed"
				break
			}
		}
	}
	if reason != "" {
		p.MarkContoursStale(reason)
	}
}
