package geom

import (
	"fmt"
	"math"
)

// TraverseAdjustment is a horizontal coordinate adjustment, not an observation
// network solution. Points contains the adjusted leg endpoints, excluding start.
// Corrections are per course; Points includes their cumulative effect.
type TraverseAdjustment struct {
	Start, Target            Point
	Method                   string
	Length                   float64
	CorrectionE, CorrectionN float64
	Misclosure               float64
	RelativePrecision        *float64 // nil when horizontal misclosure is exactly zero
	Legs                     []TraverseLeg
	Points                   []Point
}

type TraverseLeg struct {
	From, To                 string
	Length, DeltaE, DeltaN   float64
	CorrectionE, CorrectionN float64
}

// AdjustTraverse implements the compass (Bowditch) and transit rules described
// in USACE EM 1110-1-1005. Compass weights both axes by horizontal course length;
// transit weights each axis by its absolute departure/latitude respectively.
// Elevations are preserved: these rules do not define a levelling adjustment.
func AdjustTraverse(points []Point, target Point, method string) (TraverseAdjustment, error) {
	r := TraverseAdjustment{Method: method, Target: target}
	if method != "compass" && method != "transit" {
		return r, fmt.Errorf("unknown adjustment method %q", method)
	}
	if len(points) < 2 {
		return r, fmt.Errorf("traverse has no legs")
	}
	r.Start = points[0]
	if !FinitePoint(target) {
		return r, fmt.Errorf("non-finite closing coordinates")
	}
	var totalE, totalN float64
	for i, p := range points {
		if !FinitePoint(p) {
			return r, fmt.Errorf("non-finite traverse point %q", p.ID)
		}
		if i == 0 {
			continue
		}
		previous := points[i-1]
		de, dn := p.Easting-previous.Easting, p.Northing-previous.Northing
		length := math.Hypot(de, dn)
		if length == 0 || !Finite(length) {
			return r, fmt.Errorf("invalid zero-length or overflowing leg %s to %s", previous.ID, p.ID)
		}
		r.Legs = append(r.Legs, TraverseLeg{From: previous.ID, To: p.ID, Length: length, DeltaE: de, DeltaN: dn})
		r.Length += length
		totalE += math.Abs(de)
		totalN += math.Abs(dn)
	}
	last := points[len(points)-1]
	r.CorrectionE = target.Easting - last.Easting
	r.CorrectionN = target.Northing - last.Northing
	r.Misclosure = math.Hypot(r.CorrectionE, r.CorrectionN)
	if !Finite(r.Length) || !Finite(r.Misclosure) {
		return r, fmt.Errorf("traverse exceeds numeric range")
	}
	// Cardinal bearings produce tiny sin/cos artefacts; do not use those as
	// measured components to distribute a real cross-axis misclosure.
	axisTolerance := r.Length * 1e-12
	if method == "transit" && (totalE <= axisTolerance && math.Abs(r.CorrectionE) > axisTolerance || totalN <= axisTolerance && math.Abs(r.CorrectionN) > axisTolerance) {
		return r, fmt.Errorf("transit correction has no course component on the misclosed axis")
	}
	if r.Misclosure > 0 {
		ratio := r.Length / r.Misclosure
		if Finite(ratio) {
			r.RelativePrecision = &ratio
		}
	}
	var ce, cn float64
	for i := range r.Legs {
		leg := &r.Legs[i]
		we, wn := leg.Length/r.Length, leg.Length/r.Length
		if method == "transit" {
			we, wn = 0, 0
			if totalE > axisTolerance {
				we = math.Abs(leg.DeltaE) / totalE
			}
			if totalN > axisTolerance {
				wn = math.Abs(leg.DeltaN) / totalN
			}
		}
		leg.CorrectionE = r.CorrectionE * we
		leg.CorrectionN = r.CorrectionN * wn
		ce += leg.CorrectionE
		cn += leg.CorrectionN
		p := points[i+1]
		p.Easting += ce
		p.Northing += cn
		if i == len(r.Legs)-1 {
			p.Easting = target.Easting
			p.Northing = target.Northing
		}
		if !FinitePoint(p) {
			return TraverseAdjustment{}, fmt.Errorf("adjusted coordinates exceed numeric range")
		}
		r.Points = append(r.Points, p)
	}
	return r, nil
}

func Finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func FinitePoint(p Point) bool {
	return Finite(p.Easting) && Finite(p.Northing) && (p.Elevation == nil || Finite(*p.Elevation))
}
