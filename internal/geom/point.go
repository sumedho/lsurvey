package geom

import "math"

type Point struct {
	ID          string   `json:"id"`
	Easting     float64  `json:"easting"`
	Northing    float64  `json:"northing"`
	Elevation   *float64 `json:"elevation,omitempty"`
	Code        string   `json:"code,omitempty"`
	Description string   `json:"description,omitempty"`
}

type InverseResult struct {
	Azimuth            Angle
	HorizontalDistance float64
	SlopeDistance      *float64
	DeltaNorthing      float64
	DeltaEasting       float64
	DeltaElevation     *float64
	GradePercent       *float64
}

type AngleResult struct {
	Inside  Angle
	Outside Angle
}

type CloseResult struct {
	Area      float64
	Perimeter float64
	Misclose  InverseResult
}

type PointPair struct {
	Source Point
	Target Point
}

type SimilarityTransformResult struct {
	DX              float64
	DY              float64
	DZ              *float64
	RotationDegrees float64
	Scale           float64
	HorizontalRMS   float64
	VerticalRMS     *float64
	PairCount       int
	VerticalPairs   int
}

func Inverse(from, to Point) InverseResult {
	dn := to.Northing - from.Northing
	de := to.Easting - from.Easting
	hd := math.Hypot(dn, de)
	az := AngleFromRadians(math.Atan2(de, dn))
	result := InverseResult{
		Azimuth:            az,
		HorizontalDistance: hd,
		DeltaNorthing:      dn,
		DeltaEasting:       de,
	}
	if from.Elevation != nil && to.Elevation != nil {
		dz := *to.Elevation - *from.Elevation
		sd := math.Hypot(hd, dz)
		result.DeltaElevation = &dz
		result.SlopeDistance = &sd
		if hd != 0 {
			grade := dz / hd * 100
			result.GradePercent = &grade
		}
	}
	return result
}

func AngleBetween(a, vertex, b Point) (AngleResult, bool) {
	v1n := a.Northing - vertex.Northing
	v1e := a.Easting - vertex.Easting
	v2n := b.Northing - vertex.Northing
	v2e := b.Easting - vertex.Easting
	len1 := math.Hypot(v1n, v1e)
	len2 := math.Hypot(v2n, v2e)
	if len1 == 0 || len2 == 0 {
		return AngleResult{}, false
	}
	dot := v1n*v2n + v1e*v2e
	cosTheta := dot / (len1 * len2)
	cosTheta = math.Max(-1, math.Min(1, cosTheta))
	insideDeg := math.Acos(cosTheta) * RadToDeg
	return AngleResult{
		Inside:  AngleFromDegrees(insideDeg),
		Outside: AngleFromDegrees(360 - insideDeg),
	}, true
}

func Close(points []Point) (CloseResult, bool) {
	if len(points) < 3 {
		return CloseResult{}, false
	}
	var twiceArea float64
	var perimeter float64
	for i := range points {
		j := (i + 1) % len(points)
		twiceArea += points[i].Easting*points[j].Northing - points[j].Easting*points[i].Northing
		perimeter += Inverse(points[i], points[j]).HorizontalDistance
	}
	misclose := Inverse(points[len(points)-1], points[0])
	return CloseResult{
		Area:      math.Abs(twiceArea) / 2,
		Perimeter: perimeter,
		Misclose:  misclose,
	}, true
}

func FitSimilarityTransform(pairs []PointPair) (SimilarityTransformResult, bool) {
	if len(pairs) < 2 {
		return SimilarityTransformResult{}, false
	}
	var sourceE, sourceN, targetE, targetN float64
	for _, pair := range pairs {
		sourceE += pair.Source.Easting
		sourceN += pair.Source.Northing
		targetE += pair.Target.Easting
		targetN += pair.Target.Northing
	}
	count := float64(len(pairs))
	sourceE /= count
	sourceN /= count
	targetE /= count
	targetN /= count

	var denominator, numeratorA, numeratorB float64
	for _, pair := range pairs {
		de := pair.Source.Easting - sourceE
		dn := pair.Source.Northing - sourceN
		te := pair.Target.Easting - targetE
		tn := pair.Target.Northing - targetN
		denominator += de*de + dn*dn
		numeratorA += de*te + dn*tn
		numeratorB += dn*te - de*tn
	}
	if denominator < 1e-12 {
		return SimilarityTransformResult{}, false
	}
	a := numeratorA / denominator
	b := numeratorB / denominator
	scale := math.Hypot(a, b)
	if scale < 1e-12 {
		return SimilarityTransformResult{}, false
	}

	result := SimilarityTransformResult{
		DX:              targetE - a*sourceE - b*sourceN,
		DY:              targetN - a*sourceN + b*sourceE,
		RotationDegrees: math.Atan2(b, a) * RadToDeg,
		Scale:           scale,
		PairCount:       len(pairs),
	}
	var horizontalResidualSquares float64
	var elevationDiffs []float64
	for _, pair := range pairs {
		fittedE := result.DX + a*pair.Source.Easting + b*pair.Source.Northing
		fittedN := result.DY + a*pair.Source.Northing - b*pair.Source.Easting
		de := pair.Target.Easting - fittedE
		dn := pair.Target.Northing - fittedN
		horizontalResidualSquares += de*de + dn*dn
		if pair.Source.Elevation != nil && pair.Target.Elevation != nil {
			elevationDiffs = append(elevationDiffs, *pair.Target.Elevation-*pair.Source.Elevation)
		}
	}
	result.HorizontalRMS = math.Sqrt(horizontalResidualSquares / count)
	if len(elevationDiffs) > 0 {
		var dz float64
		for _, diff := range elevationDiffs {
			dz += diff
		}
		dz /= float64(len(elevationDiffs))
		var residualSquares float64
		for _, diff := range elevationDiffs {
			residual := diff - dz
			residualSquares += residual * residual
		}
		zrms := math.Sqrt(residualSquares / float64(len(elevationDiffs)))
		result.DZ = &dz
		result.VerticalRMS = &zrms
		result.VerticalPairs = len(elevationDiffs)
	}
	return result, true
}

func Radiate(from Point, azimuth Angle, horizontalDistance float64, elevationDelta *float64, id, code string) Point {
	n := from.Northing + horizontalDistance*math.Cos(azimuth.Radians())
	e := from.Easting + horizontalDistance*math.Sin(azimuth.Radians())
	var z *float64
	if from.Elevation != nil && elevationDelta != nil {
		v := *from.Elevation + *elevationDelta
		z = &v
	}
	return Point{ID: id, Northing: n, Easting: e, Elevation: z, Code: code}
}

func Radiate3D(from Point, azimuth Angle, slopeDistance float64, zenith Angle, id, code string) Point {
	horizontalDistance := slopeDistance * math.Sin(zenith.Radians())
	elevationDelta := slopeDistance * math.Cos(zenith.Radians())
	return Radiate(from, azimuth, horizontalDistance, &elevationDelta, id, code)
}

func Midpoint(a, b Point, id, code string) Point {
	p := Point{
		ID:       id,
		Northing: (a.Northing + b.Northing) / 2,
		Easting:  (a.Easting + b.Easting) / 2,
		Code:     code,
	}
	if a.Elevation != nil && b.Elevation != nil {
		z := (*a.Elevation + *b.Elevation) / 2
		p.Elevation = &z
	}
	return p
}

func Offset(a, b Point, offset, chainage float64, id, code string) Point {
	inv := Inverse(a, b)
	base := Radiate(a, inv.Azimuth, chainage, nil, id, code)
	return Radiate(base, inv.Azimuth.Add(AngleFromDegrees(90)), offset, nil, id, code)
}

func ShiftPoint(p Point, deltaE, deltaN float64, deltaZ *float64) Point {
	p.Easting += deltaE
	p.Northing += deltaN
	if deltaZ != nil && p.Elevation != nil {
		v := *p.Elevation + *deltaZ
		p.Elevation = &v
	}
	return p
}

func RotatePoint(p Point, origin Point, angle Angle) Point {
	de := p.Easting - origin.Easting
	dn := p.Northing - origin.Northing
	cosA := math.Cos(angle.Radians())
	sinA := math.Sin(angle.Radians())
	rotE := de*cosA + dn*sinA
	rotN := dn*cosA - de*sinA
	p.Easting = origin.Easting + rotE
	p.Northing = origin.Northing + rotN
	return p
}

func LineIntersection(a1, a2, b1, b2 Point, id, code string) (Point, bool) {
	x1, y1 := a1.Easting, a1.Northing
	x2, y2 := a2.Easting, a2.Northing
	x3, y3 := b1.Easting, b1.Northing
	x4, y4 := b2.Easting, b2.Northing
	den := (x1-x2)*(y3-y4) - (y1-y2)*(x3-x4)
	if math.Abs(den) < 1e-12 {
		return Point{}, false
	}
	px := ((x1*y2-y1*x2)*(x3-x4) - (x1-x2)*(x3*y4-y3*x4)) / den
	py := ((x1*y2-y1*x2)*(y3-y4) - (y1-y2)*(x3*y4-y3*x4)) / den
	return Point{ID: id, Northing: py, Easting: px, Code: code}, true
}

func BearingBearingIntersection(p1 Point, az1 Angle, p2 Point, az2 Angle, id, code string) (Point, bool) {
	a2 := Radiate(p1, az1, 1, nil, "", "")
	b2 := Radiate(p2, az2, 1, nil, "", "")
	return LineIntersection(p1, a2, p2, b2, id, code)
}

func BearingDistanceIntersection(p1 Point, az Angle, center Point, radius float64, choose string, id, code string) (Point, bool) {
	dn := math.Cos(az.Radians())
	de := math.Sin(az.Radians())
	fn := p1.Northing - center.Northing
	fe := p1.Easting - center.Easting
	b := 2 * (dn*fn + de*fe)
	c := fn*fn + fe*fe - radius*radius
	discriminant := b*b - 4*c
	if discriminant < -1e-9 {
		return Point{}, false
	}
	if discriminant < 0 {
		discriminant = 0
	}
	root := math.Sqrt(discriminant)
	t1 := (-b + root) / 2
	t2 := (-b - root) / 2
	t := t1
	if choose == "near" || choose == "left" {
		if math.Abs(t2) < math.Abs(t1) {
			t = t2
		}
	} else if choose == "far" || choose == "right" {
		if math.Abs(t2) > math.Abs(t1) {
			t = t2
		}
	}
	return Radiate(p1, az, t, nil, id, code), true
}

func ResectionByBearings(points [3]Point, bearings [3]Angle, id, code string) (Point, bool) {
	var a00, a01, a11, b0, b1 float64
	for i, pt := range points {
		az := bearings[i].Opposite()
		de := math.Sin(az.Radians())
		dn := math.Cos(az.Radians())
		normalE := -dn
		normalN := de
		c := normalE*pt.Easting + normalN*pt.Northing
		a00 += normalE * normalE
		a01 += normalE * normalN
		a11 += normalN * normalN
		b0 += normalE * c
		b1 += normalN * c
	}
	det := a00*a11 - a01*a01
	if math.Abs(det) < 1e-12 {
		return Point{}, false
	}
	e := (b0*a11 - b1*a01) / det
	n := (a00*b1 - a01*b0) / det
	return Point{ID: id, Easting: e, Northing: n, Code: code}, true
}

func DistanceDistanceIntersection(p1 Point, d1 float64, p2 Point, d2 float64, choose string, id, code string) (Point, bool) {
	d := Inverse(p1, p2).HorizontalDistance
	if d == 0 || d > d1+d2 || d < math.Abs(d1-d2) {
		return Point{}, false
	}
	a := (d1*d1 - d2*d2 + d*d) / (2 * d)
	h2 := d1*d1 - a*a
	if h2 < -1e-9 {
		return Point{}, false
	}
	if h2 < 0 {
		h2 = 0
	}
	h := math.Sqrt(h2)
	x0, y0 := p1.Easting, p1.Northing
	x1, y1 := p2.Easting, p2.Northing
	xm := x0 + a*(x1-x0)/d
	ym := y0 + a*(y1-y0)/d
	rx := -(y1 - y0) * (h / d)
	ry := (x1 - x0) * (h / d)
	if choose == "right" {
		rx = -rx
		ry = -ry
	}
	return Point{ID: id, Northing: ym + ry, Easting: xm + rx, Code: code}, true
}
