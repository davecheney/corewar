//go:build gopher_badge

// Command corewar-badge runs the direct-to-ST7789 Core War demonstration on
// the Gopher Badge. It intentionally has no framebuffer: ui only emits
// changed core cells and status fields, and display.FillRect sends each
// changed rectangle straight to the panel over SPI.
package main

import (
	"image/color"
	"log"
	"machine"
	"time"

	"github.com/davecheney/corewar/mars"
	"github.com/davecheney/corewar/ui"
	"github.com/davecheney/corewar/warriors"
	"tinygo.org/x/drivers"
	"tinygo.org/x/drivers/st7789"
)

const frameInterval = time.Second / 60

// badgePalette uses pure blue and pure red for the combatants. It is kept in
// the badge backend so the desktop retains its teal/red presentation.
var badgePalette = func() [ui.NumColors]ui.RGB {
	p := ui.Palette
	p[ui.ColorEmpty] = ui.RGB{R: 0x03, G: 0x03, B: 0x03}
	const combatantIntensity = 0xe6 // 90% of the ST7789 channel range
	p[ui.ColorTeal] = ui.RGB{R: 0, G: 0, B: combatantIntensity}
	p[ui.ColorRed] = ui.RGB{R: combatantIntensity, G: 0, B: 0}
	for i := range ui.RampLen {
		// Preserve the fade curve but cap its brightest shade at 90% too.
		blue := ui.Color(8 + i)
		red := blue + ui.RampLen
		brightness := uint8(uint16(ui.Palette[blue].B) * combatantIntensity / 0xff)
		p[blue] = ui.RGB{B: brightness}
		p[red] = ui.RGB{R: brightness}
	}
	return p
}()

type display struct{ lcd *st7789.Device }

func (d display) FillRect(x, y, w, h int, c ui.Color) {
	rgb := badgePalette[c]
	// Rectangle coordinates are already constrained by ui's 320x240 layout.
	if err := d.lcd.FillRectangle(int16(x), int16(y), int16(w), int16(h),
		color.RGBA{R: rgb.R, G: rgb.G, B: rgb.B, A: 0xff}); err != nil {
		panic(err)
	}
}

func main() {
	machine.SPI0.Configure(machine.SPIConfig{
		Frequency: 32_000_000,
		Mode:      0,
	})

	lcd := st7789.New(machine.SPI0,
		machine.TFT_RST,
		machine.TFT_WRX,
		machine.TFT_CS,
		machine.TFT_BACKLIGHT,
	)
	lcd.Configure(st7789.Config{
		// The Gopher Badge panel is physically 240×320. Rotation270
		// exposes it to the UI as the intended 320×240 landscape view.
		Height:   320,
		Rotation: drivers.Rotation270,
	})

	// The UI only repaints changed rectangles after this initial clear.
	out := display{lcd: &lcd}
	out.FillRect(0, 0, ui.ScreenWidth, ui.ScreenHeight, ui.ColorBackground)

	cfg := mars.DefaultConfig()
	// Allocate the core and display state while the heap is empty, before
	// assembly leaves it fragmented.
	game := ui.NewGame(cfg, uint64(time.Now().UnixNano()))

	// Only the two warriors fighting are assembled and held in memory; any
	// that fails to assemble is left out when first picked.
	roster, err := warriors.NewRoster(warriors.FS, cfg)
	if err != nil {
		log.Fatal(err)
	}
	if roster.Len() < 2 {
		log.Fatal("corewar: need at least 2 warriors")
	}
	game.Warn = func(err error) { println("corewar: skipped:", err.Error()) }
	game.Start(roster)
	input := newButtons()

	tick := time.NewTicker(frameInterval)
	for range tick.C {
		game.Update(input.poll())
		game.Draw(out)
	}
}

type buttons struct {
	a, b, up, down, left, right machine.Pin
}

func newButtons() buttons {
	b := buttons{
		a: machine.BUTTON_A, b: machine.BUTTON_B,
		up: machine.BUTTON_UP, down: machine.BUTTON_DOWN,
		left: machine.BUTTON_LEFT, right: machine.BUTTON_RIGHT,
	}
	for _, pin := range [...]machine.Pin{b.a, b.b, b.up, b.down, b.left, b.right} {
		pin.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	return b
}

func (b buttons) poll() ui.Buttons {
	var held ui.Buttons
	// Gopher Badge buttons are active low.
	if !b.a.Get() {
		held |= ui.ButtonSkip
	}
	if !b.b.Get() {
		held |= ui.ButtonPause
	}
	if !b.up.Get() {
		held |= ui.ButtonFaster
	}
	if !b.down.Get() {
		held |= ui.ButtonSlower
	}
	if !b.left.Get() {
		held |= ui.ButtonNewLeft
	}
	if !b.right.Get() {
		held |= ui.ButtonNewRight
	}
	return held
}
