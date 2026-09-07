package geom

import "math"

const eps = 1e-9

func PolygonInvalid(points []Point) bool {
	return PolygonZeroArea(points) || PolygonSelfIntersects(points)
}

func PolygonZeroArea(points []Point) bool {
	return math.Abs(polygonTwiceArea(points)) < eps
}

func PolygonSelfIntersects(points []Point) bool {
	for i := range points {
		a, b := points[i], points[(i+1)%len(points)]
		for j := i + 1; j < len(points); j++ {
			if polygonSegmentsAdjacent(i, j, len(points)) {
				continue
			}
			c, d := points[j], points[(j+1)%len(points)]
			if SegmentsCross(a, b, c, d) {
				return true
			}
		}
	}
	return false
}

func polygonSegmentsAdjacent(i, j, n int) bool {
	return i == j || (i+1)%n == j || (j+1)%n == i
}

func SegmentsCross(a, b, c, d Point) bool {
	orient := func(p, q, r Point) float64 {
		return (q.Easting-p.Easting)*(r.Northing-p.Northing) - (q.Northing-p.Northing)*(r.Easting-p.Easting)
	}
	onSegment := func(p, q, r Point) bool {
		return q.Easting >= math.Min(p.Easting, r.Easting)-eps && q.Easting <= math.Max(p.Easting, r.Easting)+eps &&
			q.Northing >= math.Min(p.Northing, r.Northing)-eps && q.Northing <= math.Max(p.Northing, r.Northing)+eps
	}
	o1, o2, o3, o4 := orient(a, b, c), orient(a, b, d), orient(c, d, a), orient(c, d, b)
	if math.Abs(o1) <= eps && onSegment(a, c, b) || math.Abs(o2) <= eps && onSegment(a, d, b) ||
		math.Abs(o3) <= eps && onSegment(c, a, d) || math.Abs(o4) <= eps && onSegment(c, b, d) {
		return true
	}
	return (o1 > 0) != (o2 > 0) && (o3 > 0) != (o4 > 0)
}

func polygonTwiceArea(points []Point) float64 {
	area := 0.0
	for i := range points {
		a, b := points[i], points[(i+1)%len(points)]
		ae, an := a.Easting-points[0].Easting, a.Northing-points[0].Northing
		be, bn := b.Easting-points[0].Easting, b.Northing-points[0].Northing
		area += ae*bn - be*an
	}
	return area
}
