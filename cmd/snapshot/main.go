// Command snapshot runs the demo headless and writes frames as PNGs, for
// checking the layout and colours without a window. Like corewar, it takes
// optional directories or Redcode files of warriors as arguments.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/davecheney/corewar/mars"
	"github.com/davecheney/corewar/ui"
	"github.com/davecheney/corewar/warriors"
)

func main() {
	seed := flag.Uint64("seed", 1, "random seed")
	frames := flag.String("frames", "60,600,1800", "comma separated frame numbers to capture")
	speed := flag.Int("speed", ui.DefaultSpeed, "index into ui.Speeds")
	scale := flag.Int("scale", 2, "pixel scale")
	out := flag.String("o", "frame-%05d.png", "output file pattern")
	builtin := flag.Bool("builtin", false, "include the built-in warriors when paths are given")
	flag.Parse()

	cfg := mars.DefaultConfig()
	ws, err := warriors.Load(cfg)
	if err != nil && (flag.NArg() == 0 || *builtin) {
		fmt.Fprintf(os.Stderr, "snapshot: warning:\n%v\n", err)
	}
	if flag.NArg() > 0 {
		found, err := warriors.LoadPaths(flag.Args(), cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "snapshot: warning:\n%v\n", err)
		}
		if !*builtin {
			ws = nil
		}
		ws = append(ws, found...)
	}
	if len(ws) < 2 {
		log.Fatalf("need at least 2 warriors, found %d", len(ws))
	}
	g := ui.New(ws, cfg, *seed)
	for range *speed - ui.DefaultSpeed {
		g.Update(ui.ButtonFaster)
		g.Update(0)
	}
	for range ui.DefaultSpeed - *speed {
		g.Update(ui.ButtonSlower)
		g.Update(0)
	}

	want := map[int]bool{}
	last := 0
	for _, f := range strings.Split(*frames, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil {
			log.Fatal(err)
		}
		want[n] = true
		last = max(last, n)
	}

	var buf ui.Buffer
	for frame := 1; frame <= last; frame++ {
		g.Update(0)
		g.Draw(&buf)
		if want[frame] {
			name := fmt.Sprintf(*out, frame)
			if err := write(name, &buf, *scale); err != nil {
				log.Fatal(err)
			}
			fmt.Println(name)
		}
	}
}

func write(name string, buf *ui.Buffer, scale int) error {
	pal := make(color.Palette, ui.NumColors)
	for i, c := range ui.Palette {
		pal[i] = color.RGBA{c.R, c.G, c.B, 0xff}
	}
	img := image.NewPaletted(image.Rect(0, 0, ui.ScreenWidth*scale, ui.ScreenHeight*scale), pal)
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			img.SetColorIndex(x, y, uint8(buf[y/scale*ui.ScreenWidth+x/scale]))
		}
	}
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
