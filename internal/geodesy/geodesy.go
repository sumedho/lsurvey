package geodesy

import (
	"fmt"
	"math"
)

type Datum string

const (
	GDA94   Datum = "GDA94"
	GDA2020 Datum = "GDA2020"
)

type System string

const (
	MGA94System         System = "MGA94"
	MGA2020System       System = "MGA2020"
	GDA94Geographic     System = "GDA94_GEOGRAPHIC"
	GDA2020Geographic   System = "GDA2020_GEOGRAPHIC"
	Conformal           string = "conformal"
	ConformalDistortion string = "conformal_and_distortion"
)

type Geographic struct {
	Datum     Datum
	Latitude  float64
	Longitude float64
	Elevation *float64
}

type Grid struct {
	Datum     Datum
	Zone      int
	Easting   float64
	Northing  float64
	Elevation *float64
}

type Coordinate struct {
	Geographic *Geographic
	Grid       *Grid
}

type Transformer interface {
	Forward(Geographic) (Geographic, error)
	Reverse(Geographic) (Geographic, error)
}

type Request struct {
	Source     Coordinate
	Target     System
	TargetZone int
	Transform  Transformer
}

func DatumForSystem(system System) (Datum, error) {
	switch system {
	case MGA94System, GDA94Geographic:
		return GDA94, nil
	case MGA2020System, GDA2020Geographic:
		return GDA2020, nil
	default:
		return "", fmt.Errorf("unknown coordinate system %q", system)
	}
}

func Convert(request Request) (Coordinate, error) {
	var geographic Geographic
	switch {
	case request.Source.Geographic != nil && request.Source.Grid == nil:
		geographic = *request.Source.Geographic
		if err := ValidateGeographic(geographic); err != nil {
			return Coordinate{}, err
		}
	case request.Source.Grid != nil && request.Source.Geographic == nil:
		var err error
		geographic, err = GridToGeographic(*request.Source.Grid)
		if err != nil {
			return Coordinate{}, err
		}
	default:
		return Coordinate{}, fmt.Errorf("source must contain exactly one coordinate representation")
	}
	targetDatum, err := DatumForSystem(request.Target)
	if err != nil {
		return Coordinate{}, err
	}
	if geographic.Datum != targetDatum {
		if request.Transform == nil {
			return Coordinate{}, fmt.Errorf("a datum transformation model is required")
		}
		switch {
		case geographic.Datum == GDA94 && targetDatum == GDA2020:
			geographic, err = request.Transform.Forward(geographic)
		case geographic.Datum == GDA2020 && targetDatum == GDA94:
			geographic, err = request.Transform.Reverse(geographic)
		default:
			err = fmt.Errorf("unsupported datum transformation %s to %s", geographic.Datum, targetDatum)
		}
		if err != nil {
			return Coordinate{}, err
		}
	}
	switch request.Target {
	case GDA94Geographic, GDA2020Geographic:
		return Coordinate{Geographic: &geographic}, nil
	case MGA94System, MGA2020System:
		grid, err := GeographicToGrid(geographic, request.TargetZone)
		if err != nil {
			return Coordinate{}, err
		}
		return Coordinate{Grid: &grid}, nil
	default:
		return Coordinate{}, fmt.Errorf("unknown target system %q", request.Target)
	}
}

const (
	grs80A        = 6378137.0
	grs80InvF     = 298.257222101
	mgaScale      = 0.9996
	falseEasting  = 500000.0
	falseNorthing = 10000000.0
)

var (
	grs80F  = 1 / grs80InvF
	grs80N  = grs80F / (2 - grs80F)
	grs80E2 = grs80F * (2 - grs80F)
	grs80E  = math.Sqrt(grs80E2)
	tmA     = meridianRadius()
	alpha   = forwardCoefficients()
	beta    = reverseCoefficients()
)

func ValidateGeographic(c Geographic) error {
	if c.Datum != GDA94 && c.Datum != GDA2020 {
		return fmt.Errorf("unsupported datum %q", c.Datum)
	}
	if math.IsNaN(c.Latitude) || math.IsNaN(c.Longitude) || math.IsInf(c.Latitude, 0) || math.IsInf(c.Longitude, 0) {
		return fmt.Errorf("latitude and longitude must be finite")
	}
	if c.Latitude < -61 || c.Latitude > -8 || c.Longitude < 93 || c.Longitude > 174 {
		return fmt.Errorf("coordinate is outside the supported Australian extent")
	}
	return nil
}

func GeographicToGrid(c Geographic, zone int) (Grid, error) {
	if err := ValidateGeographic(c); err != nil {
		return Grid{}, err
	}
	if err := validateZone(zone); err != nil {
		return Grid{}, err
	}
	phi := radians(c.Latitude)
	lam := radians(c.Longitude - centralMeridian(zone))
	tau := math.Tan(phi)
	taup := conformalTau(tau)
	xip := math.Atan2(taup, math.Cos(lam))
	etap := math.Asinh(math.Sin(lam) / math.Sqrt(taup*taup+math.Cos(lam)*math.Cos(lam)))
	xi, eta := xip, etap
	for j := 1; j <= 6; j++ {
		xi += alpha[j-1] * math.Sin(2*float64(j)*xip) * math.Cosh(2*float64(j)*etap)
		eta += alpha[j-1] * math.Cos(2*float64(j)*xip) * math.Sinh(2*float64(j)*etap)
	}
	return Grid{
		Datum: c.Datum, Zone: zone,
		Easting:   falseEasting + mgaScale*tmA*eta,
		Northing:  falseNorthing + mgaScale*tmA*xi,
		Elevation: c.Elevation,
	}, nil
}

func MGAZoneForLongitude(longitude float64) (int, error) {
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < 93 || longitude > 174 {
		return 0, fmt.Errorf("longitude is outside the supported Australian extent")
	}
	zone := int(math.Floor((longitude+180)/6)) + 1
	if zone > 59 {
		zone = 59
	}
	if err := validateZone(zone); err != nil {
		return 0, err
	}
	return zone, nil
}

func GridToGeographic(c Grid) (Geographic, error) {
	if c.Datum != GDA94 && c.Datum != GDA2020 {
		return Geographic{}, fmt.Errorf("unsupported datum %q", c.Datum)
	}
	if err := validateZone(c.Zone); err != nil {
		return Geographic{}, err
	}
	if math.IsNaN(c.Easting) || math.IsNaN(c.Northing) || math.IsInf(c.Easting, 0) || math.IsInf(c.Northing, 0) {
		return Geographic{}, fmt.Errorf("easting and northing must be finite")
	}
	xi := (c.Northing - falseNorthing) / (mgaScale * tmA)
	eta := (c.Easting - falseEasting) / (mgaScale * tmA)
	xip, etap := xi, eta
	for j := 1; j <= 6; j++ {
		xip -= beta[j-1] * math.Sin(2*float64(j)*xi) * math.Cosh(2*float64(j)*eta)
		etap -= beta[j-1] * math.Cos(2*float64(j)*xi) * math.Sinh(2*float64(j)*eta)
	}
	taup := math.Sin(xip) / math.Sqrt(math.Sinh(etap)*math.Sinh(etap)+math.Cos(xip)*math.Cos(xip))
	tau := taup
	for i := 0; i < 8; i++ {
		got := conformalTau(tau)
		delta := (taup - got) * (1 + (1-grs80E2)*tau*tau) /
			((1 - grs80E2) * math.Sqrt(1+tau*tau) * math.Sqrt(1+got*got))
		tau += delta
		if math.Abs(delta) < 1e-14 {
			break
		}
	}
	result := Geographic{
		Datum:     c.Datum,
		Latitude:  degrees(math.Atan(tau)),
		Longitude: centralMeridian(c.Zone) + degrees(math.Atan2(math.Sinh(etap), math.Cos(xip))),
		Elevation: c.Elevation,
	}
	if err := ValidateGeographic(result); err != nil {
		return Geographic{}, err
	}
	return result, nil
}

func validateZone(zone int) error {
	if zone < 46 || zone > 59 {
		return fmt.Errorf("MGA zone must be from 46 to 59")
	}
	return nil
}

func centralMeridian(zone int) float64 {
	return float64(zone*6 - 183)
}

func conformalTau(tau float64) float64 {
	sigma := math.Sinh(grs80E * math.Atanh(grs80E*tau/math.Sqrt(1+tau*tau)))
	return tau*math.Sqrt(1+sigma*sigma) - sigma*math.Sqrt(1+tau*tau)
}

func meridianRadius() float64 {
	n := grs80N
	return grs80A / (1 + n) * (1 + n*n/4 + math.Pow(n, 4)/64 + math.Pow(n, 6)/256)
}

func forwardCoefficients() [6]float64 {
	n := grs80N
	return [6]float64{
		n/2 - 2*math.Pow(n, 2)/3 + 5*math.Pow(n, 3)/16 + 41*math.Pow(n, 4)/180 - 127*math.Pow(n, 5)/288 + 7891*math.Pow(n, 6)/37800,
		13*math.Pow(n, 2)/48 - 3*math.Pow(n, 3)/5 + 557*math.Pow(n, 4)/1440 + 281*math.Pow(n, 5)/630 - 1983433*math.Pow(n, 6)/1935360,
		61*math.Pow(n, 3)/240 - 103*math.Pow(n, 4)/140 + 15061*math.Pow(n, 5)/26880 + 167603*math.Pow(n, 6)/181440,
		49561*math.Pow(n, 4)/161280 - 179*math.Pow(n, 5)/168 + 6601661*math.Pow(n, 6)/7257600,
		34729*math.Pow(n, 5)/80640 - 3418889*math.Pow(n, 6)/1995840,
		212378941 * math.Pow(n, 6) / 319334400,
	}
}

func reverseCoefficients() [6]float64 {
	n := grs80N
	return [6]float64{
		n/2 - 2*math.Pow(n, 2)/3 + 37*math.Pow(n, 3)/96 - math.Pow(n, 4)/360 - 81*math.Pow(n, 5)/512 + 96199*math.Pow(n, 6)/604800,
		math.Pow(n, 2)/48 + math.Pow(n, 3)/15 - 437*math.Pow(n, 4)/1440 + 46*math.Pow(n, 5)/105 - 1118711*math.Pow(n, 6)/3870720,
		17*math.Pow(n, 3)/480 - 37*math.Pow(n, 4)/840 - 209*math.Pow(n, 5)/4480 + 5569*math.Pow(n, 6)/90720,
		4397*math.Pow(n, 4)/161280 - 11*math.Pow(n, 5)/504 - 830251*math.Pow(n, 6)/7257600,
		4583*math.Pow(n, 5)/161280 - 108847*math.Pow(n, 6)/3991680,
		20648693 * math.Pow(n, 6) / 638668800,
	}
}

func radians(value float64) float64 { return value * math.Pi / 180 }
func degrees(value float64) float64 { return value * 180 / math.Pi }
