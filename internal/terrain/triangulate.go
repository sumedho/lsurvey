package terrain

import (
	"fmt"
	"math"

	"github.com/fogleman/delaunay"
)

func triangulate(points []vertex) ([]triangle, error) {
	if collinear(points) {
		return nil, fmt.Errorf("elevated points are collinear")
	}

	delaunayPoints := make([]delaunay.Point, len(points))
	for i, point := range points {
		delaunayPoints[i] = delaunay.Point{X: point.E, Y: point.N}
	}
	triangulation, err := delaunay.Triangulate(delaunayPoints)
	if err != nil {
		return nil, err
	}

	var out []triangle
	for i := 0; i+2 < len(triangulation.Triangles); i += 3 {
		tri := triangle{
			A: triangulation.Triangles[i],
			B: triangulation.Triangles[i+1],
			C: triangulation.Triangles[i+2],
		}
		if tri.A < 0 || tri.B < 0 || tri.C < 0 || tri.A >= len(points) || tri.B >= len(points) || tri.C >= len(points) {
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
