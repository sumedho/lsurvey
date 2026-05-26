package tui

import "fmt"

// ACIColor is the nominal screen RGB representation of one AutoCAD Color Index entry.
type ACIColor struct {
	Index int
	Name  string
	R     uint8
	G     uint8
	B     uint8
}

func (c ACIColor) Hex() string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

func (c ACIColor) RGB() string {
	return fmt.Sprintf("RGB(%d,%d,%d)", c.R, c.G, c.B)
}

func allACIColors() []ACIColor {
	colors := make([]ACIColor, 0, 255)
	primary := []ACIColor{
		{Index: 1, Name: "Red", R: 255},
		{Index: 2, Name: "Yellow", R: 255, G: 255},
		{Index: 3, Name: "Green", G: 255},
		{Index: 4, Name: "Cyan", G: 255, B: 255},
		{Index: 5, Name: "Blue", B: 255},
		{Index: 6, Name: "Magenta", R: 255, B: 255},
		{Index: 7, Name: "White/Black", R: 255, G: 255, B: 255},
		{Index: 8, Name: "Dark Gray", R: 128, G: 128, B: 128},
		{Index: 9, Name: "Light Gray", R: 192, G: 192, B: 192},
	}
	colors = append(colors, primary...)

	hues := [][3]uint8{
		{255, 0, 0}, {255, 63, 0}, {255, 127, 0}, {255, 191, 0},
		{255, 255, 0}, {191, 255, 0}, {127, 255, 0}, {63, 255, 0},
		{0, 255, 0}, {0, 255, 63}, {0, 255, 127}, {0, 255, 191},
		{0, 255, 255}, {0, 191, 255}, {0, 127, 255}, {0, 63, 255},
		{0, 0, 255}, {63, 0, 255}, {127, 0, 255}, {191, 0, 255},
		{255, 0, 255}, {255, 0, 191}, {255, 0, 127}, {255, 0, 63},
	}
	levels := []uint8{255, 165, 127, 76, 38}
	for hueIndex, hue := range hues {
		for shade, level := range levels {
			scaled := [3]uint8{
				uint8(int(hue[0]) * int(level) / 255),
				uint8(int(hue[1]) * int(level) / 255),
				uint8(int(hue[2]) * int(level) / 255),
			}
			base := 10 + hueIndex*10 + shade*2
			colors = append(colors,
				ACIColor{Index: base, R: scaled[0], G: scaled[1], B: scaled[2]},
				ACIColor{
					Index: base + 1,
					R:     uint8((int(scaled[0]) + int(level)) / 2),
					G:     uint8((int(scaled[1]) + int(level)) / 2),
					B:     uint8((int(scaled[2]) + int(level)) / 2),
				},
			)
		}
	}
	grays := []uint8{51, 80, 105, 130, 190, 255}
	for i, value := range grays {
		colors = append(colors, ACIColor{Index: 250 + i, R: value, G: value, B: value})
	}
	return colors
}

func aciColor(index int) (ACIColor, bool) {
	if index < 1 || index > 255 {
		return ACIColor{}, false
	}
	return allACIColors()[index-1], true
}
