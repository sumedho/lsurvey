package terrain

import (
	"fmt"
	"math"

	"lsurvey/internal/project"
)

type breakline struct {
	ID        string
	FeatureID string
	From      string
	To        string
	A         int
	B         int
}

func validateBreaklines(p *project.Project, opts Options, points []vertex) ([]breakline, error) {
	if !opts.UseBreakline {
		return nil, nil
	}
	index := map[string]int{}
	byCoordinate := map[string]int{}
	for i, pt := range points {
		index[pt.ID] = i
		byCoordinate[coordKey(pt.E, pt.N)] = i
	}
	ids := opts.BreaklineIDs
	if len(ids) == 0 {
		clipCodes := map[string]bool{}
		for _, code := range append(append([]string(nil), opts.BoundaryCodes...), opts.ExclusionCodes...) {
			clipCodes[code] = true
		}
		for _, feature := range p.SortedFeatures() {
			if feature.Kind == project.FeaturePolygon || clipCodes[feature.Code] {
				continue
			}
			ids = append(ids, feature.ID)
		}
	}
	var out []breakline
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		feature, ok := p.Features[id]
		if !ok || feature.Kind == project.FeaturePolygon {
			return nil, fmt.Errorf("breakline %q not found", id)
		}
		for _, segment := range p.FeatureSegments(feature) {
			a, ok := index[segment.From]
			if !ok {
				if pt, found := p.Points[segment.From]; found && pt.Elevation != nil {
					a, ok = byCoordinate[coordKey(pt.Easting, pt.Northing)]
				}
			}
			if !ok {
				return nil, fmt.Errorf("breakline %q from point %q has no elevation", id, segment.From)
			}
			b, ok := index[segment.To]
			if !ok {
				if pt, found := p.Points[segment.To]; found && pt.Elevation != nil {
					b, ok = byCoordinate[coordKey(pt.Easting, pt.Northing)]
				}
			}
			if !ok {
				return nil, fmt.Errorf("breakline %q to point %q has no elevation", id, segment.To)
			}
			if a == b {
				return nil, fmt.Errorf("breakline %q has identical endpoints", id)
			}
			out = append(out, breakline{
				ID: fmt.Sprintf("%s:%d", id, segment.Index+1), FeatureID: id,
				From: segment.From, To: segment.To, A: a, B: b,
			})
		}
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if sharesEndpoint(out[i], out[j]) {
				continue
			}
			if segmentsIntersect(points[out[i].A], points[out[i].B], points[out[j].A], points[out[j].B]) {
				return nil, fmt.Errorf("breaklines %q and %q cross", out[i].ID, out[j].ID)
			}
		}
	}
	return out, nil
}

func sharesEndpoint(a, b breakline) bool {
	return a.A == b.A || a.A == b.B || a.B == b.A || a.B == b.B
}

func insertConstraintEdges(points []vertex, tris []triangle, breaklines []breakline) ([]triangle, error) {
	protected := map[edge]bool{}
	for _, bl := range breaklines {
		target := normEdge(bl.A, bl.B)
		for i := 0; i < len(points)*len(points); i++ {
			if hasEdge(tris, target) {
				protected[target] = true
				break
			}
			crossing, ok := findCrossingEdge(points, tris, target, protected)
			if !ok {
				return nil, fmt.Errorf("could not insert breakline %q", bl.ID)
			}
			var flipped bool
			tris, flipped = flipEdge(points, tris, crossing)
			if !flipped {
				return nil, fmt.Errorf("could not recover breakline %q", bl.ID)
			}
		}
		if !hasEdge(tris, target) {
			return nil, fmt.Errorf("could not insert breakline %q", bl.ID)
		}
		protected[target] = true
	}
	return tris, nil
}

func hasEdge(tris []triangle, e edge) bool {
	for _, tri := range tris {
		if normEdge(tri.A, tri.B) == e || normEdge(tri.B, tri.C) == e || normEdge(tri.C, tri.A) == e {
			return true
		}
	}
	return false
}

func findCrossingEdge(points []vertex, tris []triangle, target edge, protected map[edge]bool) (edge, bool) {
	seen := map[edge]bool{}
	for _, tri := range tris {
		for _, e := range []edge{normEdge(tri.A, tri.B), normEdge(tri.B, tri.C), normEdge(tri.C, tri.A)} {
			if seen[e] || protected[e] || e.A == target.A || e.A == target.B || e.B == target.A || e.B == target.B {
				continue
			}
			seen[e] = true
			if segmentsIntersect(points[target.A], points[target.B], points[e.A], points[e.B]) {
				return e, true
			}
		}
	}
	return edge{}, false
}

func flipEdge(points []vertex, tris []triangle, e edge) ([]triangle, bool) {
	var adjacent []int
	var opposite []int
	for i, tri := range tris {
		if triangleHasEdge(tri, e) {
			adjacent = append(adjacent, i)
			opposite = append(opposite, oppositeVertex(tri, e))
		}
	}
	if len(adjacent) != 2 {
		return tris, false
	}
	x, y := opposite[0], opposite[1]
	if x == y || x < 0 || y < 0 {
		return tris, false
	}
	if !segmentsIntersect(points[e.A], points[e.B], points[x], points[y]) {
		return tris, false
	}
	newTris := make([]triangle, 0, len(tris))
	for i, tri := range tris {
		if i == adjacent[0] || i == adjacent[1] {
			continue
		}
		newTris = append(newTris, tri)
	}
	newTris = append(newTris, triangle{A: x, B: y, C: e.A}, triangle{A: y, B: x, C: e.B})
	return newTris, true
}

func triangleHasEdge(tri triangle, e edge) bool {
	return normEdge(tri.A, tri.B) == e || normEdge(tri.B, tri.C) == e || normEdge(tri.C, tri.A) == e
}

func oppositeVertex(tri triangle, e edge) int {
	for _, v := range []int{tri.A, tri.B, tri.C} {
		if v != e.A && v != e.B {
			return v
		}
	}
	return -1
}

func segmentsIntersect(a, b, c, d vertex) bool {
	o1 := orient(a, b, c)
	o2 := orient(a, b, d)
	o3 := orient(c, d, a)
	o4 := orient(c, d, b)
	if math.Abs(o1) <= eps && onSegment(a, c, b) {
		return true
	}
	if math.Abs(o2) <= eps && onSegment(a, d, b) {
		return true
	}
	if math.Abs(o3) <= eps && onSegment(c, a, d) {
		return true
	}
	if math.Abs(o4) <= eps && onSegment(c, b, d) {
		return true
	}
	return (o1 > 0) != (o2 > 0) && (o3 > 0) != (o4 > 0)
}

func onSegment(a, p, b vertex) bool {
	return p.E >= math.Min(a.E, b.E)-eps && p.E <= math.Max(a.E, b.E)+eps &&
		p.N >= math.Min(a.N, b.N)-eps && p.N <= math.Max(a.N, b.N)+eps
}
