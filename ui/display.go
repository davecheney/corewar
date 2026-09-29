// Package ui draws a Core War battle on a 320×240 display and runs the
// attract-mode demo. It never keeps a framebuffer: it only ever asks the
// display to fill rectangles, and only where something has changed, so a
// SPI panel can be driven directly.
package ui

// Screen geometry.
const (
	ScreenWidth  = 320
	ScreenHeight = 240

	StatusHeight = 40 // the status bar occupies y 0..39

	CellWidth  = 2
	CellHeight = 4
	CoreCols   = ScreenWidth / CellWidth                    // 160
	CoreRows   = (ScreenHeight - StatusHeight) / CellHeight // 50
	CoreCells  = CoreCols * CoreRows                        // 8000
)

// Display is all the UI needs from a screen.
type Display interface {
	FillRect(x, y, w, h int, c Color)
}

// drawText draws s at (x, y) in colour c on background bg, with runs of
// lit pixels in each row drawn as single rectangles.
func drawText(d Display, x, y int, s []byte, c, bg Color) {
	d.FillRect(x, y, len(s)*glyphWidth, glyphHeight, bg)
	for i, ch := range s {
		if ch < firstGlyph || ch > lastGlyph {
			ch = '?'
		}
		g := glyphs[int(ch-firstGlyph)*glyphHeight:][:glyphHeight]
		gx := x + i*glyphWidth
		for row := range glyphHeight {
			bits := g[row]
			for col := 0; col < glyphWidth; {
				if bits&(0x40>>col) == 0 {
					col++
					continue
				}
				start := col
				for col < glyphWidth && bits&(0x40>>col) != 0 {
					col++
				}
				d.FillRect(gx+start, y+row, col-start, 1, c)
			}
		}
	}
}
