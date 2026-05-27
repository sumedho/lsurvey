package geodesy

import (
	"fmt"
	"math"
	"strings"

	"lsurvey/internal/geom"
)

const geographicSecondPrecision = 5

func ParseLatitude(value string) (float64, error) {
	return parseGeographicComponent(value, "latitude", "N", "S", 90)
}

func ParseLongitude(value string) (float64, error) {
	return parseGeographicComponent(value, "longitude", "E", "W", 180)
}

func FormatLatitude(value float64) string {
	return formatGeographicComponent(value, "N", "S")
}

func FormatLongitude(value float64) string {
	return formatGeographicComponent(value, "E", "W")
}

func parseGeographicComponent(value, name, positiveHemisphere, negativeHemisphere string, maxDegrees float64) (float64, error) {
	input := strings.TrimSpace(value)
	if input == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	fields := strings.Fields(strings.ToUpper(input))
	if len(fields) > 2 {
		return 0, fmt.Errorf("invalid %s %q", name, value)
	}
	hemisphere := ""
	number := fields[0]
	if len(fields) == 2 {
		switch {
		case fields[0] == positiveHemisphere || fields[0] == negativeHemisphere:
			hemisphere, number = fields[0], fields[1]
		case fields[1] == positiveHemisphere || fields[1] == negativeHemisphere:
			number, hemisphere = fields[0], fields[1]
		default:
			return 0, fmt.Errorf("invalid %s hemisphere in %q", name, value)
		}
	}
	sign := 1.0
	explicitSign := false
	if strings.HasPrefix(number, "-") {
		sign = -1
		explicitSign = true
		number = strings.TrimPrefix(number, "-")
	} else if strings.HasPrefix(number, "+") {
		explicitSign = true
		number = strings.TrimPrefix(number, "+")
	}
	if number == "" {
		return 0, fmt.Errorf("invalid %s %q", name, value)
	}
	angle, err := geom.ParseAngle(number)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	degrees := angle.Degrees()
	if degrees > maxDegrees {
		return 0, fmt.Errorf("%s must be no greater than %.0f degrees", name, maxDegrees)
	}
	if hemisphere != "" {
		wantSign := 1.0
		if hemisphere == negativeHemisphere {
			wantSign = -1
		}
		if explicitSign && sign != wantSign {
			return 0, fmt.Errorf("%s sign conflicts with hemisphere in %q", name, value)
		}
		sign = wantSign
	}
	return sign * degrees, nil
}

func formatGeographicComponent(value float64, positiveHemisphere, negativeHemisphere string) string {
	hemisphere := positiveHemisphere
	if value < 0 {
		hemisphere = negativeHemisphere
	}
	return geom.AngleFromDegrees(math.Abs(value)).FormatDMS(geographicSecondPrecision) + " " + hemisphere
}
