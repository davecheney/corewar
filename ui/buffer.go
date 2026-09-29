package ui

// Buffer is a Display backed by one palette index per pixel. The desktop
// presents it as a paletted texture; tests and snapshots read it directly.
type Buffer [ScreenWidth * ScreenHeight]Color

// FillRect implements Display, clipping to the screen.
func (b *Buffer) FillRect(x, y, w, h int, c Color) {
	x0, y0, x1, y1 := max(x, 0), max(y, 0), min(x+w, ScreenWidth), min(y+h, ScreenHeight)
	for yy := y0; yy < y1; yy++ {
		row := b[yy*ScreenWidth:][x0:x1]
		for i := range row {
			row[i] = c
		}
	}
}
