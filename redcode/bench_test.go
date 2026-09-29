package redcode_test

import (
	"path"
	"strings"
	"testing"

	"github.com/davecheney/corewar/mars"
	"github.com/davecheney/corewar/redcode"
	"github.com/davecheney/corewar/warriors"
)

// BenchmarkAssemble assembles each built-in warrior. Allocations matter as
// much as time: on the Gopher Badge the garbage from assembling the roster
// competes with the core for a heap of under 256 KB.
func BenchmarkAssemble(b *testing.B) {
	cfg := mars.DefaultConfig()
	for _, src := range warriors.All() {
		name := strings.TrimSuffix(path.Base(src.File), path.Ext(src.File))
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(src.Code)))
			for b.Loop() {
				redcode.Assemble(src.Code, cfg)
			}
		})
	}
}

// BenchmarkAssembleAll assembles the whole built-in roster, as the demo
// does at start up.
func BenchmarkAssembleAll(b *testing.B) {
	cfg := mars.DefaultConfig()
	srcs := warriors.All()
	var n int64
	for _, src := range srcs {
		n += int64(len(src.Code))
	}
	b.ReportAllocs()
	b.SetBytes(n)
	for b.Loop() {
		for _, src := range srcs {
			redcode.Assemble(src.Code, cfg)
		}
	}
}

// BenchmarkFor measures FOR/ROF expansion, which rewrites the line list.
func BenchmarkFor(b *testing.B) {
	if !redcode.ForEnabled {
		b.Skip("FOR/ROF is disabled")
	}
	src := ";redcode\ni FOR 50\nx&i DAT #i, #x&i\nj FOR 2\nDAT #j, #i\nROF\nROF\n"
	cfg := mars.DefaultConfig()
	cfg.MaxLength = 200
	b.ReportAllocs()
	for b.Loop() {
		if _, err := redcode.Assemble(src, cfg); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkComparison measures the comparison and logical operators.
func BenchmarkComparison(b *testing.B) {
	src := "DAT #(CORESIZE == 8000 && MAXLENGTH >= 100 || !(VERSION < 80)), #0\n"
	cfg := mars.DefaultConfig()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := redcode.Assemble(src, cfg); err != nil {
			b.Fatal(err)
		}
	}
}
