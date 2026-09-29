// Command corewar runs the Core War attract-mode demo in a desktop window.
//
// Usage:
//
//	corewar [-builtin] [-seed n] [path ...]
//
// With no paths the built-in warriors fight. Each path may be a directory,
// walked recursively for .red files, or a single Redcode file; the warriors
// found there fight instead of the built-ins, or alongside them with
// -builtin. Other files are ignored, and .red files that are not valid
// ICWS'94 Redcode are skipped with a warning. ;assert comments are
// ignored.
//
// Keys: N or Enter starts a new pairing, Left / Right replace the left or
// right warrior, Space or P pauses, + / - (or Up / Down) change speed, Esc
// or Q quits. The winner of each match stays on against a new challenger.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/davecheney/corewar/mars"
	"github.com/davecheney/corewar/platform/sdl"
	"github.com/davecheney/corewar/ui"
	"github.com/davecheney/corewar/warriors"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("corewar: ")
	seed := flag.Uint64("seed", uint64(time.Now().UnixNano()), "random seed")
	builtin := flag.Bool("builtin", false, "include the built-in warriors when paths are given")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: corewar [flags] [dir|file ...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg := mars.DefaultConfig()
	ws, err := load(cfg, flag.Args(), *builtin)
	if err != nil {
		log.Fatal(err)
	}
	if err := sdl.Run("Core War", ui.New(ws, cfg, *seed)); err != nil {
		log.Fatal(err)
	}
}

// load gathers the warriors to fight, warning about any that are skipped.
func load(cfg mars.Config, paths []string, builtin bool) ([]*mars.Warrior, error) {
	var ws []*mars.Warrior
	if len(paths) == 0 || builtin {
		b, err := warriors.Load(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "corewar: warning:\n%v\n", err)
		}
		ws = append(ws, b...)
	}
	if len(paths) > 0 {
		found, err := warriors.LoadPaths(paths, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "corewar: warning:\n%v\n", err)
		}
		ws = append(ws, found...)
	}
	if len(ws) < 2 {
		return nil, fmt.Errorf("need at least 2 warriors, found %d", len(ws))
	}
	return ws, nil
}
