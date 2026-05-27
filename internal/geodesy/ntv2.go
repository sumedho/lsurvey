package geodesy

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type NTv2Grid struct {
	Model    string
	subgrids []subgrid
}

type subgrid struct {
	Name              string
	South, North      float64
	East, West        float64
	LatitudeInterval  float64
	LongitudeInterval float64
	Columns           int
	Nodes             []shift
}

type shift struct {
	Latitude  float64
	Longitude float64
}

func LoadNTv2(path, model string) (*NTv2Grid, error) {
	if model != Conformal && model != ConformalDistortion {
		return nil, fmt.Errorf("unknown transformation model %q", model)
	}
	filename := strings.ToLower(filepath.Base(path))
	switch model {
	case ConformalDistortion:
		if !strings.Contains(filename, "conformal_and_distortion") {
			return nil, fmt.Errorf("selected conformal-and-distortion model requires the official conformal_and_distortion .gsb file")
		}
	case Conformal:
		if !strings.Contains(filename, "conformal") || strings.Contains(filename, "conformal_and_distortion") {
			return nil, fmt.Errorf("selected conformal model requires the official conformal-only .gsb file")
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	grid, err := ReadNTv2(f)
	if err != nil {
		return nil, err
	}
	grid.Model = model
	return grid, nil
}

func ReadNTv2(r io.Reader) (*NTv2Grid, error) {
	header, err := readRecords(r, 11)
	if err != nil {
		return nil, fmt.Errorf("read NTv2 header: %w", err)
	}
	order, err := gridByteOrder(header)
	if err != nil {
		return nil, err
	}
	count := intValue(header["NUM_FILE"], order)
	if count <= 0 {
		return nil, fmt.Errorf("NTv2 file contains no subgrids")
	}
	grid := &NTv2Grid{}
	for i := 0; i < count; i++ {
		records, err := readRecords(r, 11)
		if err != nil {
			return nil, fmt.Errorf("read NTv2 subgrid %d: %w", i+1, err)
		}
		item := subgrid{
			Name:  strings.TrimSpace(string(records["SUB_NAME"])),
			South: floatValue(records["S_LAT"], order), North: floatValue(records["N_LAT"], order),
			East: floatValue(records["E_LONG"], order), West: floatValue(records["W_LONG"], order),
			LatitudeInterval:  floatValue(records["LAT_INC"], order),
			LongitudeInterval: floatValue(records["LONG_INC"], order),
		}
		nodeCount := intValue(records["GS_COUNT"], order)
		if item.LatitudeInterval <= 0 || item.LongitudeInterval <= 0 || nodeCount <= 0 {
			return nil, fmt.Errorf("invalid NTv2 subgrid %q", item.Name)
		}
		item.Columns = int(math.Round((item.West-item.East)/item.LongitudeInterval)) + 1
		rows := int(math.Round((item.North-item.South)/item.LatitudeInterval)) + 1
		if item.Columns <= 1 || rows <= 1 || item.Columns*rows != nodeCount {
			return nil, fmt.Errorf("invalid NTv2 node geometry in subgrid %q", item.Name)
		}
		item.Nodes = make([]shift, nodeCount)
		for j := range item.Nodes {
			var values [4]float32
			if err := binary.Read(r, order, &values); err != nil {
				return nil, fmt.Errorf("read NTv2 nodes in subgrid %q: %w", item.Name, err)
			}
			item.Nodes[j] = shift{Latitude: float64(values[0]), Longitude: float64(values[1])}
		}
		grid.subgrids = append(grid.subgrids, item)
	}
	return grid, nil
}

func (g *NTv2Grid) Forward(c Geographic) (Geographic, error) {
	if c.Datum != GDA94 {
		return Geographic{}, fmt.Errorf("forward NTv2 transformation requires GDA94 input")
	}
	delta, err := g.interpolate(c)
	if err != nil {
		return Geographic{}, err
	}
	return Geographic{
		Datum: GDA2020, Latitude: c.Latitude + delta.Latitude/3600,
		Longitude: c.Longitude - delta.Longitude/3600, Elevation: c.Elevation,
	}, nil
}

func (g *NTv2Grid) Reverse(c Geographic) (Geographic, error) {
	if c.Datum != GDA2020 {
		return Geographic{}, fmt.Errorf("reverse NTv2 transformation requires GDA2020 input")
	}
	guess := Geographic{Datum: GDA94, Latitude: c.Latitude, Longitude: c.Longitude, Elevation: c.Elevation}
	for i := 0; i < 10; i++ {
		forward, err := g.Forward(guess)
		if err != nil {
			return Geographic{}, err
		}
		dLat := c.Latitude - forward.Latitude
		dLon := c.Longitude - forward.Longitude
		guess.Latitude += dLat
		guess.Longitude += dLon
		if math.Abs(dLat)+math.Abs(dLon) < 1e-13 {
			return guess, nil
		}
	}
	return Geographic{}, fmt.Errorf("NTv2 reverse transformation did not converge")
}

func (g *NTv2Grid) interpolate(c Geographic) (shift, error) {
	lat := c.Latitude * 3600
	lonWest := -c.Longitude * 3600
	var selected *subgrid
	for i := range g.subgrids {
		item := &g.subgrids[i]
		if lat >= item.South && lat <= item.North && lonWest >= item.East && lonWest <= item.West {
			if selected == nil || item.LatitudeInterval < selected.LatitudeInterval {
				selected = item
			}
		}
	}
	if selected == nil {
		return shift{}, fmt.Errorf("coordinate is outside the selected NTv2 grid coverage")
	}
	x := (lonWest - selected.East) / selected.LongitudeInterval
	y := (lat - selected.South) / selected.LatitudeInterval
	col := min(int(math.Floor(x)), selected.Columns-2)
	rows := len(selected.Nodes) / selected.Columns
	row := min(int(math.Floor(y)), rows-2)
	fx, fy := x-float64(col), y-float64(row)
	if x == float64(selected.Columns-1) {
		fx = 1
	}
	if y == float64(rows-1) {
		fy = 1
	}
	a := selected.Nodes[row*selected.Columns+col]
	b := selected.Nodes[row*selected.Columns+col+1]
	c1 := selected.Nodes[(row+1)*selected.Columns+col]
	d := selected.Nodes[(row+1)*selected.Columns+col+1]
	return shift{
		Latitude:  bilinear(a.Latitude, b.Latitude, c1.Latitude, d.Latitude, fx, fy),
		Longitude: bilinear(a.Longitude, b.Longitude, c1.Longitude, d.Longitude, fx, fy),
	}, nil
}

func bilinear(a, b, c, d, x, y float64) float64 {
	return a*(1-x)*(1-y) + b*x*(1-y) + c*(1-x)*y + d*x*y
}

func readRecords(r io.Reader, count int) (map[string][]byte, error) {
	records := make(map[string][]byte, count)
	for i := 0; i < count; i++ {
		label := make([]byte, 8)
		value := make([]byte, 8)
		if _, err := io.ReadFull(r, label); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(r, value); err != nil {
			return nil, err
		}
		records[strings.TrimSpace(string(label))] = value
	}
	return records, nil
}

func gridByteOrder(header map[string][]byte) (binary.ByteOrder, error) {
	value, ok := header["NUM_OREC"]
	if !ok {
		return nil, fmt.Errorf("invalid NTv2 header: NUM_OREC missing")
	}
	if intValue(value, binary.LittleEndian) == 11 {
		return binary.LittleEndian, nil
	}
	if intValue(value, binary.BigEndian) == 11 {
		return binary.BigEndian, nil
	}
	return nil, fmt.Errorf("invalid NTv2 header: unsupported byte order")
}

func intValue(value []byte, order binary.ByteOrder) int {
	return int(int32(order.Uint32(value[:4])))
}

func floatValue(value []byte, order binary.ByteOrder) float64 {
	return math.Float64frombits(order.Uint64(value))
}
