package ui

// Color is an index into Palette. Displays map it to whatever pixel format
// they use: an INDEX8 texture on the desktop, RGB565 on the ST7789.
type Color uint8

const (
	ColorBackground Color = iota
	ColorEmpty            // a core cell no warrior has touched
	ColorDivider
	ColorText
	ColorTextDim
	ColorTeal // text in the teal warrior's colour
	ColorRed  // text in the red warrior's colour

	rampTeal Color = 8 // first of RampLen shades, brightest first
	rampRed  Color = rampTeal + RampLen

	// NumColors is the number of palette entries in use.
	NumColors = int(rampRed + RampLen)
)

// RampLen is how many shades a cell fades through, from just executed down
// to the terminal colour that marks its owner.
const RampLen = 8

// RGB is a 24-bit colour.
type RGB struct{ R, G, B uint8 }

// RGB565 packs c into the ST7789's native 16-bit format.
func (c RGB) RGB565() uint16 {
	return uint16(c.R>>3)<<11 | uint16(c.G>>2)<<5 | uint16(c.B>>3)
}

// Palette holds the colour of every Color.
var Palette [NumColors]RGB

// The two combatants share saturation and lightness and differ only in hue:
// a light teal and the red that matches it.
const (
	hueTeal    = 174
	hueRed     = 0
	saturation = 0.82
)

// rampLightness runs from a near-white flash for an executed cell, through
// the base colour for a freshly written one, down to the dim terminal shade.
var rampLightness = [RampLen]float64{0.88, 0.63, 0.54, 0.45, 0.37, 0.30, 0.24, 0.19}

func init() {
	Palette[ColorBackground] = RGB{0, 0, 0}
	Palette[ColorEmpty] = RGB{0x1c, 0x1e, 0x22}
	Palette[ColorDivider] = RGB{0x40, 0x44, 0x48}
	Palette[ColorText] = RGB{0xe8, 0xe8, 0xe8}
	Palette[ColorTextDim] = RGB{0x88, 0x8c, 0x90}
	Palette[ColorTeal] = hsl(hueTeal, saturation, rampLightness[1])
	Palette[ColorRed] = hsl(hueRed, saturation, rampLightness[1])
	for i := range RampLen {
		Palette[rampTeal+Color(i)] = hsl(hueTeal, saturation, rampLightness[i])
		Palette[rampRed+Color(i)] = hsl(hueRed, saturation, rampLightness[i])
	}
}

// WarriorColor is the text colour of warrior w (0 teal, 1 red).
func WarriorColor(w int) Color {
	if w == 0 {
		return ColorTeal
	}
	return ColorRed
}

func hsl(h, s, l float64) RGB {
	c := (1 - abs(2*l-1)) * s
	hp := h / 60
	x := c * (1 - abs(mod2(hp)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g = c, x
	case hp < 2:
		r, g = x, c
	case hp < 3:
		g, b = c, x
	case hp < 4:
		g, b = x, c
	case hp < 5:
		r, b = x, c
	default:
		r, b = c, x
	}
	m := l - c/2
	return RGB{to8(r + m), to8(g + m), to8(b + m)}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func mod2(v float64) float64 {
	for v >= 2 {
		v -= 2
	}
	return v
}

func to8(v float64) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 1:
		return 255
	}
	return uint8(v*255 + 0.5)
}
