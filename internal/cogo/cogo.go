package cogo

import (
	"fmt"

	"lsurvey/internal/project"
)

type Result struct {
	Message             string
	Created             []string
	Updated             []string
	Changed             bool
	StaleContoursReason string
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
		if result.StaleContoursReason != "" {
			p.MarkContoursStale(result.StaleContoursReason)
		}
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

func changed(result Result) Result {
	result.Changed = true
	return result
}

func staleContours(result Result, reason string) Result {
	result.Changed = true
	result.StaleContoursReason = reason
	return result
}

func staleContoursIf(result Result, reason string, stale bool) Result {
	if stale {
		return staleContours(result, reason)
	}
	return changed(result)
}
