package terrain

import (
	"fmt"
	"math"
	"sort"

	"lsurvey/internal/project"
)

type clipRegions struct {
	Boundary       []project.ContourVertex
	Exclusions     [][]project.ContourVertex
	BoundaryLines  []string
	ExclusionLines []string
}

type clipEdge struct {
	ID   string
	From string
	To   string
}

func resolveClipRegions(p *project.Project, opts Options) (clipRegions, error) {
	var out clipRegions
	if len(opts.BoundaryCodes) > 0 {
		rings, ids, err := ringsByCodes(p, opts.BoundaryCodes, "boundary")
		if err != nil {
			return out, err
		}
		if len(rings) != 1 {
			return out, fmt.Errorf("boundary codes must define exactly one closed ring")
		}
		out.Boundary = rings[0]
		out.BoundaryLines = ids
	}
	if len(opts.ExclusionCodes) > 0 {
		rings, ids, err := ringsByCodes(p, opts.ExclusionCodes, "exclusion")
		if err != nil {
			return out, err
		}
		out.Exclusions = rings
		out.ExclusionLines = ids
	}
	for i, ring := range out.Exclusions {
		if len(out.Boundary) > 0 {
			if ringsIntersect(out.Boundary, ring) || !pointInRing(ring[0], out.Boundary, true) {
				return out, fmt.Errorf("exclusion ring %d must be inside the boundary without crossing it", i+1)
			}
		}
		for j := 0; j < i; j++ {
			if ringsIntersect(out.Exclusions[j], ring) {
				return out, fmt.Errorf("exclusion rings %d and %d cross", j+1, i+1)
			}
		}
	}
	return out, nil
}

func ringsByCodes(p *project.Project, codes []string, label string) ([][]project.ContourVertex, []string, error) {
	selected := map[string]bool{}
	for _, code := range codes {
		if code == "" {
			return nil, nil, fmt.Errorf("%s code is empty", label)
		}
		selected[code] = true
	}
	var edges []clipEdge
	var rings [][]project.ContourVertex
	var ids []string
	for _, feature := range p.SortedFeatures() {
		if !selected[feature.Code] {
			continue
		}
		ids = append(ids, feature.ID)
		if feature.Kind == project.FeaturePolygon {
			ring := make([]project.ContourVertex, 0, len(feature.PointIDs))
			for _, pointID := range feature.PointIDs {
				if _, ok := p.Points[pointID]; !ok {
					return nil, nil, fmt.Errorf("%s feature %q point %q not found", label, feature.ID, pointID)
				}
				ring = append(ring, contourVertex(p, pointID))
			}
			if len(ring) < 3 || ringSelfIntersects(ring) {
				return nil, nil, fmt.Errorf("%s polygon %q is invalid", label, feature.ID)
			}
			rings = append(rings, ring)
			continue
		}
		for _, segment := range p.FeatureSegments(feature) {
			edges = append(edges, clipEdge{
				ID: fmt.Sprintf("%s:%d", feature.ID, segment.Index+1), From: segment.From, To: segment.To,
			})
		}
	}
	if len(ids) == 0 {
		return nil, nil, fmt.Errorf("%s codes match no features", label)
	}
	assembled, err := assembleRings(p, edges, label)
	return append(rings, assembled...), ids, err
}

func assembleRings(p *project.Project, lines []clipEdge, label string) ([][]project.ContourVertex, error) {
	incident := map[string][]clipEdge{}
	unused := map[string]clipEdge{}
	for _, line := range lines {
		if line.From == line.To {
			return nil, fmt.Errorf("%s line %q has identical endpoints", label, line.ID)
		}
		if _, ok := p.Points[line.From]; !ok {
			return nil, fmt.Errorf("%s line %q point %q not found", label, line.ID, line.From)
		}
		if _, ok := p.Points[line.To]; !ok {
			return nil, fmt.Errorf("%s line %q point %q not found", label, line.ID, line.To)
		}
		incident[line.From] = append(incident[line.From], line)
		incident[line.To] = append(incident[line.To], line)
		unused[line.ID] = line
	}
	for id, connected := range incident {
		if len(connected) != 2 {
			return nil, fmt.Errorf("%s linework is not closed at point %q", label, id)
		}
	}

	var rings [][]project.ContourVertex
	for _, first := range lines {
		if _, ok := unused[first.ID]; !ok {
			continue
		}
		delete(unused, first.ID)
		start := first.From
		current := first.To
		ring := []project.ContourVertex{contourVertex(p, start)}
		for current != start {
			ring = append(ring, contourVertex(p, current))
			var next *clipEdge
			for _, line := range incident[current] {
				if _, ok := unused[line.ID]; ok {
					copy := line
					next = &copy
					break
				}
			}
			if next == nil {
				return nil, fmt.Errorf("%s linework does not close", label)
			}
			delete(unused, next.ID)
			if next.From == current {
				current = next.To
			} else {
				current = next.From
			}
		}
		if len(ring) < 3 {
			return nil, fmt.Errorf("%s ring requires at least three vertices", label)
		}
		if ringSelfIntersects(ring) {
			return nil, fmt.Errorf("%s ring self-intersects", label)
		}
		rings = append(rings, ring)
	}
	return rings, nil
}

func contourVertex(p *project.Project, id string) project.ContourVertex {
	pt := p.Points[id]
	return project.ContourVertex{Easting: pt.Easting, Northing: pt.Northing}
}

func ringSelfIntersects(ring []project.ContourVertex) bool {
	for i := range ring {
		a, b := ring[i], ring[(i+1)%len(ring)]
		for j := i + 1; j < len(ring); j++ {
			if j == i || j == i+1 || i == 0 && j == len(ring)-1 {
				continue
			}
			if contourSegmentsIntersect(a, b, ring[j], ring[(j+1)%len(ring)]) {
				return true
			}
		}
	}
	return false
}

func ringsIntersect(a, b []project.ContourVertex) bool {
	for i := range a {
		for j := range b {
			if contourSegmentsIntersect(a[i], a[(i+1)%len(a)], b[j], b[(j+1)%len(b)]) {
				return true
			}
		}
	}
	return false
}

func contourSegmentsIntersect(a, b, c, d project.ContourVertex) bool {
	return segmentsIntersect(toVertex(a), toVertex(b), toVertex(c), toVertex(d))
}

func toVertex(v project.ContourVertex) vertex {
	return vertex{E: v.Easting, N: v.Northing}
}

func clipSegments(segments []segment, clip clipRegions) []segment {
	if len(clip.Boundary) == 0 && len(clip.Exclusions) == 0 {
		return segments
	}
	var out []segment
	for _, seg := range segments {
		ts := []float64{0, 1}
		for _, ring := range append([][]project.ContourVertex{clip.Boundary}, clip.Exclusions...) {
			for i := range ring {
				ts = append(ts, segmentIntersectionParameters(seg.A, seg.B, ring[i], ring[(i+1)%len(ring)])...)
			}
		}
		sort.Float64s(ts)
		ts = uniqueParameters(ts)
		for i := 0; i+1 < len(ts); i++ {
			if ts[i+1]-ts[i] <= eps {
				continue
			}
			mid := interpolate(seg.A, seg.B, (ts[i]+ts[i+1])/2)
			if len(clip.Boundary) > 0 && !pointInRing(mid, clip.Boundary, true) {
				continue
			}
			excluded := false
			for _, ring := range clip.Exclusions {
				if pointInRing(mid, ring, true) {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
			part := seg
			part.A = interpolate(seg.A, seg.B, ts[i])
			part.B = interpolate(seg.A, seg.B, ts[i+1])
			out = append(out, part)
		}
	}
	return out
}

func segmentIntersectionParameters(a, b, c, d project.ContourVertex) []float64 {
	rx, ry := b.Easting-a.Easting, b.Northing-a.Northing
	sx, sy := d.Easting-c.Easting, d.Northing-c.Northing
	cross := rx*sy - ry*sx
	qx, qy := c.Easting-a.Easting, c.Northing-a.Northing
	if math.Abs(cross) <= eps {
		if math.Abs(qx*ry-qy*rx) > eps {
			return nil
		}
		denom := rx*rx + ry*ry
		if denom <= eps {
			return nil
		}
		var out []float64
		for _, v := range []project.ContourVertex{c, d} {
			t := ((v.Easting-a.Easting)*rx + (v.Northing-a.Northing)*ry) / denom
			if t >= -eps && t <= 1+eps {
				out = append(out, math.Max(0, math.Min(1, t)))
			}
		}
		return out
	}
	t := (qx*sy - qy*sx) / cross
	u := (qx*ry - qy*rx) / cross
	if t < -eps || t > 1+eps || u < -eps || u > 1+eps {
		return nil
	}
	return []float64{math.Max(0, math.Min(1, t))}
}

func uniqueParameters(values []float64) []float64 {
	var out []float64
	for _, value := range values {
		if len(out) == 0 || math.Abs(out[len(out)-1]-value) > eps {
			out = append(out, value)
		}
	}
	return out
}

func interpolate(a, b project.ContourVertex, t float64) project.ContourVertex {
	return project.ContourVertex{
		Easting:  a.Easting + t*(b.Easting-a.Easting),
		Northing: a.Northing + t*(b.Northing-a.Northing),
	}
}

func pointInRing(point project.ContourVertex, ring []project.ContourVertex, boundaryInside bool) bool {
	inside := false
	for i := range ring {
		a, b := ring[i], ring[(i+1)%len(ring)]
		if pointOnContourSegment(point, a, b) {
			return boundaryInside
		}
		crosses := (a.Northing > point.Northing) != (b.Northing > point.Northing)
		if crosses {
			x := (b.Easting-a.Easting)*(point.Northing-a.Northing)/(b.Northing-a.Northing) + a.Easting
			if point.Easting < x {
				inside = !inside
			}
		}
	}
	return inside
}

func pointOnContourSegment(p, a, b project.ContourVertex) bool {
	cross := (b.Easting-a.Easting)*(p.Northing-a.Northing) - (b.Northing-a.Northing)*(p.Easting-a.Easting)
	if math.Abs(cross) > eps {
		return false
	}
	return p.Easting >= math.Min(a.Easting, b.Easting)-eps && p.Easting <= math.Max(a.Easting, b.Easting)+eps &&
		p.Northing >= math.Min(a.Northing, b.Northing)-eps && p.Northing <= math.Max(a.Northing, b.Northing)+eps
}
