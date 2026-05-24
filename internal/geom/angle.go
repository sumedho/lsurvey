package geom

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const DegToRad = math.Pi / 180
const RadToDeg = 180 / math.Pi

type Angle struct {
	degrees float64
}

func AngleFromDegrees(degrees float64) Angle {
	return Angle{degrees: normalizeDegrees(degrees)}
}

func AngleFromRadians(radians float64) Angle {
	return AngleFromDegrees(radians * RadToDeg)
}

func (a Angle) Degrees() float64 {
	return a.degrees
}

func (a Angle) Radians() float64 {
	return a.degrees * DegToRad
}

func (a Angle) Add(b Angle) Angle {
	return AngleFromDegrees(a.degrees + b.degrees)
}

func (a Angle) Opposite() Angle {
	return AngleFromDegrees(a.degrees + 180)
}

func ParseAngle(input string) (Angle, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return Angle{}, fmt.Errorf("empty angle")
	}
	if strings.HasSuffix(strings.ToLower(s), "d") {
		deg, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(s, "d"), "D"), 64)
		if err != nil {
			return Angle{}, fmt.Errorf("invalid decimal degrees %q", input)
		}
		return AngleFromDegrees(deg), nil
	}
	return ParseCompactDMS(s)
}

func ParseCompactDMS(input string) (Angle, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return Angle{}, fmt.Errorf("empty angle")
	}

	sign := 1.0
	if strings.HasPrefix(s, "-") {
		sign = -1
		s = strings.TrimPrefix(s, "-")
	} else if strings.HasPrefix(s, "+") {
		s = strings.TrimPrefix(s, "+")
	}

	parts := strings.Split(s, ".")
	if len(parts) > 2 || parts[0] == "" {
		return Angle{}, fmt.Errorf("invalid compact DMS %q", input)
	}
	deg, err := strconv.Atoi(parts[0])
	if err != nil {
		return Angle{}, fmt.Errorf("invalid degrees in %q", input)
	}

	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	for _, r := range frac {
		if r < '0' || r > '9' {
			return Angle{}, fmt.Errorf("invalid compact DMS %q", input)
		}
	}
	if len(frac) < 4 {
		frac += strings.Repeat("0", 4-len(frac))
	}

	minutes, _ := strconv.Atoi(frac[:2])
	secondsWhole, _ := strconv.Atoi(frac[2:4])
	if minutes >= 60 {
		return Angle{}, fmt.Errorf("minutes must be less than 60 in %q", input)
	}
	if secondsWhole >= 60 {
		return Angle{}, fmt.Errorf("seconds must be less than 60 in %q", input)
	}

	seconds := float64(secondsWhole)
	if len(frac) > 4 {
		fraction, err := strconv.ParseFloat("0."+frac[4:], 64)
		if err != nil {
			return Angle{}, fmt.Errorf("invalid fractional seconds in %q", input)
		}
		seconds += fraction
	}

	total := float64(deg) + float64(minutes)/60 + seconds/3600
	return AngleFromDegrees(sign * total), nil
}

func ParseQuadrantBearing(tokens []string) (Angle, int, error) {
	if len(tokens) < 3 {
		return Angle{}, 0, fmt.Errorf("bearing requires quadrant angle quadrant")
	}
	ns := strings.ToUpper(tokens[0])
	ew := strings.ToUpper(tokens[2])
	if (ns != "N" && ns != "S") || (ew != "E" && ew != "W") {
		return Angle{}, 0, fmt.Errorf("invalid bearing quadrants")
	}
	a, err := ParseAngle(tokens[1])
	if err != nil {
		return Angle{}, 0, err
	}
	if a.Degrees() > 90 {
		return Angle{}, 0, fmt.Errorf("quadrant bearing angle must be <= 90 degrees")
	}

	deg := a.Degrees()
	switch {
	case ns == "N" && ew == "E":
		return AngleFromDegrees(deg), 3, nil
	case ns == "S" && ew == "E":
		return AngleFromDegrees(180 - deg), 3, nil
	case ns == "S" && ew == "W":
		return AngleFromDegrees(180 + deg), 3, nil
	default:
		return AngleFromDegrees(360 - deg), 3, nil
	}
}

func (a Angle) FormatCompactDMS(precision int) string {
	d, m, s, frac := a.dmsParts(precision)
	if precision == 0 {
		return fmt.Sprintf("%d.%02d%02d", d, m, s)
	}
	return fmt.Sprintf("%d.%02d%02d%0*d", d, m, s, precision, frac)
}

func (a Angle) FormatDMS(precision int) string {
	d, m, s, frac := a.dmsParts(precision)
	if precision == 0 {
		return fmt.Sprintf("%d°%02d′%02d″", d, m, s)
	}
	return fmt.Sprintf("%d°%02d′%02d.%0*d″", d, m, s, precision, frac)
}

func (a Angle) FormatQuadrant(precision int) string {
	deg := normalizeDegrees(a.degrees)
	switch {
	case deg <= 90:
		return "N " + AngleFromDegrees(deg).FormatDMS(precision) + " E"
	case deg <= 180:
		return "S " + AngleFromDegrees(180-deg).FormatDMS(precision) + " E"
	case deg <= 270:
		return "S " + AngleFromDegrees(deg-180).FormatDMS(precision) + " W"
	default:
		return "N " + AngleFromDegrees(360-deg).FormatDMS(precision) + " W"
	}
}

func (a Angle) dmsParts(precision int) (int, int, int, int) {
	if precision < 0 {
		precision = 0
	}
	deg := normalizeDegrees(a.degrees)
	scale := math.Pow10(precision)
	total := math.Round(deg * 3600 * scale)
	d := int(total / (3600 * scale))
	total -= float64(d) * 3600 * scale
	m := int(total / (60 * scale))
	total -= float64(m) * 60 * scale
	s := int(total / scale)
	frac := int(total) - s*int(scale)
	if d == 360 {
		return 0, 0, 0, 0
	}
	return d, m, s, frac
}

func normalizeDegrees(deg float64) float64 {
	deg = math.Mod(deg, 360)
	if deg < 0 {
		deg += 360
	}
	if deg == 360 {
		return 0
	}
	return deg
}
