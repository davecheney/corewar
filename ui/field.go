package ui

import "strconv"

type align uint8

const (
	alignLeft align = iota
	alignCenter
	alignRight
)

const maxField = 16

// field is a fixed-width piece of status text that is only redrawn when
// its text or colour changes. Text is built in place so updating it every
// frame does not allocate.
type field struct {
	x, y  int
	width int // in characters
	align align

	text  [maxField]byte
	n     int
	color Color

	shown  [maxField]byte
	shownN int
	shownC Color
	valid  bool
}

func (f *field) set(c Color, parts ...string) {
	f.color = c
	f.n = 0
	for _, p := range parts {
		f.n += copy(f.text[f.n:f.width], p)
	}
}

func (f *field) setInt(c Color, prefix string, v int) {
	f.set(c, prefix)
	f.addInt(v)
}

func (f *field) add(s string) { f.n += copy(f.text[f.n:f.width], s) }

func (f *field) addInt(v int) {
	var buf [12]byte
	f.n += copy(f.text[f.n:f.width], strconv.AppendInt(buf[:0], int64(v), 10))
}

func (f *field) draw(d Display) {
	if f.valid && f.shownC == f.color && f.shownN == f.n && f.shown == f.text {
		return
	}
	d.FillRect(f.x, f.y, f.width*glyphWidth, glyphHeight, ColorBackground)
	x := f.x
	switch f.align {
	case alignCenter:
		x += (f.width - f.n) * glyphWidth / 2
	case alignRight:
		x += (f.width - f.n) * glyphWidth
	}
	drawText(d, x, f.y, f.text[:f.n], f.color, ColorBackground)
	f.shown, f.shownN, f.shownC, f.valid = f.text, f.n, f.color, true
}
