package ui

import "github.com/davecheney/corewar/mars"

// framesPerShade is how many frames a cell spends on each shade of its
// ramp, so a cell takes RampLen*framesPerShade frames to fade out.
const framesPerShade = 4

const maxAge = RampLen*framesPerShade - 1

// coreView tracks what each core cell looks like and redraws only cells
// that are fading. A touched cell goes on the decay list and is redrawn
// each time its shade changes until it reaches its owner's terminal colour,
// when it leaves the list and is not drawn again until touched.
type coreView struct {
	owner  [CoreCells]uint8 // 0 untouched, else warrior+1
	age    [CoreCells]uint8
	drawn  [CoreCells]Color // what is on screen now
	inList [CoreCells]bool
	list   [CoreCells]uint16
	n      int

	cleared bool // the whole core area needs clearing before drawing
}

func (v *coreView) reset() {
	v.owner = [CoreCells]uint8{}
	v.inList = [CoreCells]bool{}
	for i := range v.drawn {
		v.drawn[i] = ColorEmpty
	}
	v.n = 0
	v.cleared = true
}

// touch is the mars.TouchFunc.
func (v *coreView) touch(addr, w int, k mars.TouchKind) {
	if addr >= CoreCells {
		return
	}
	v.owner[addr] = uint8(w + 1)
	age := uint8(framesPerShade) // a write starts at the base colour
	if k == mars.Exec {
		age = 0 // an execute flashes
	}
	if !v.inList[addr] {
		v.inList[addr] = true
		v.list[v.n] = uint16(addr)
		v.n++
		v.age[addr] = age
	} else if age < v.age[addr] {
		v.age[addr] = age
	}
}

func (v *coreView) color(addr int) Color {
	ramp := rampTeal
	if v.owner[addr] == 2 {
		ramp = rampRed
	}
	return ramp + Color(int(v.age[addr])/framesPerShade)
}

// draw redraws changed cells and ages everything on the decay list by one
// frame.
func (v *coreView) draw(d Display) {
	if v.cleared {
		d.FillRect(0, StatusHeight, ScreenWidth, ScreenHeight-StatusHeight, ColorEmpty)
		v.cleared = false
	}
	for i := 0; i < v.n; {
		addr := int(v.list[i])
		if c := v.color(addr); c != v.drawn[addr] {
			d.FillRect(addr%CoreCols*CellWidth, StatusHeight+addr/CoreCols*CellHeight, CellWidth, CellHeight, c)
			v.drawn[addr] = c
		}
		if v.age[addr] >= maxAge {
			v.inList[addr] = false
			v.n--
			v.list[i] = v.list[v.n]
			continue
		}
		v.age[addr]++
		i++
	}
}
