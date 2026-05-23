package terrain

import (
	"fmt"
	"math"
)

func triangulate(points []vertex) ([]triangle, error) {
	if collinear(points) {
		return nil, fmt.Errorf("elevated points are collinear")
	}
	all := make([]vertex, 0, len(points)+3)
	all = append(all, points...)
	st := superTriangle(points)
	all = append(all, st...)
	s0 := len(points)
	tris := []triangle{{A: s0, B: s0 + 1, C: s0 + 2}}
	for i := range points {
		var polygon []edge
		var kept []triangle
		for _, tri := range tris {
			if inCircumcircle(all[i], all[tri.A], all[tri.B], all[tri.C]) {
				polygon = toggleBoundaryEdge(polygon, normEdge(tri.A, tri.B))
				polygon = toggleBoundaryEdge(polygon, normEdge(tri.B, tri.C))
				polygon = toggleBoundaryEdge(polygon, normEdge(tri.C, tri.A))
			} else {
				kept = append(kept, tri)
			}
		}
		for _, e := range polygon {
			if math.Abs(orient(all[e.A], all[e.B], all[i])) <= eps {
				continue
			}
			kept = append(kept, triangle{A: e.A, B: e.B, C: i})
		}
		tris = kept
	}
	var out []triangle
	for _, tri := range tris {
		if tri.A >= len(points) || tri.B >= len(points) || tri.C >= len(points) {
			continue
		}
		if math.Abs(orient(points[tri.A], points[tri.B], points[tri.C])) <= eps {
			continue
		}
		out = append(out, tri)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("could not triangulate points")
	}
	return out, nil
}

func superTriangle(points []vertex) []vertex {
	minE, maxE := points[0].E, points[0].E
	minN, maxN := points[0].N, points[0].N
	for _, p := range points[1:] {
		minE = math.Min(minE, p.E)
		maxE = math.Max(maxE, p.E)
		minN = math.Min(minN, p.N)
		maxN = math.Max(maxN, p.N)
	}
	d := math.Max(maxE-minE, maxN-minN)
	if d == 0 {
		d = 1
	}
	midE := (minE + maxE) / 2
	midN := (minN + maxN) / 2
	d *= 100
	return []vertex{
		{E: midE - 2*d, N: midN - d},
		{E: midE, N: midN + 2*d},
		{E: midE + 2*d, N: midN - d},
	}
}

func toggleBoundaryEdge(edges []edge, e edge) []edge {
	for i, existing := range edges {
		if existing == e {
			return append(edges[:i], edges[i+1:]...)
		}
	}
	return append(edges, e)
}

func inCircumcircle(p, a, b, c vertex) bool {
	if orient(a, b, c) < 0 {
		b, c = c, b
	}
	ax := a.E - p.E
	ay := a.N - p.N
	bx := b.E - p.E
	by := b.N - p.N
	cx := c.E - p.E
	cy := c.N - p.N
	det := (ax*ax+ay*ay)*(bx*cy-cx*by) -
		(bx*bx+by*by)*(ax*cy-cx*ay) +
		(cx*cx+cy*cy)*(ax*by-bx*ay)
	return det > eps
}

func collinear(points []vertex) bool {
	for i := 2; i < len(points); i++ {
		if math.Abs(orient(points[0], points[1], points[i])) > eps {
			return false
		}
	}
	return true
}

func orient(a, b, c vertex) float64 {
	return (b.E-a.E)*(c.N-a.N) - (b.N-a.N)*(c.E-a.E)
}

func normEdge(a, b int) edge {
	if a > b {
		a, b = b, a
	}
	return edge{A: a, B: b}
}
