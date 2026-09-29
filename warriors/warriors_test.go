package warriors

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/davecheney/corewar/mars"
	"github.com/davecheney/corewar/redcode"
)

// needsOpponent lists warriors that kill themselves when run alone.
var needsOpponent = map[string]bool{
	// Written for an 8192-cell core (their ;assert is ignored); their step
	// sizes eventually bomb their own code in 8000 cells.
	"griffin2.red": true,
	"twimp.red":    true,
}

func TestSoloSurvival(t *testing.T) {
	cfg := mars.DefaultConfig()
	for _, src := range All() {
		if needsOpponent[src.File] {
			continue
		}
		w, err := redcode.Assemble(src.Code, cfg)
		if err != nil {
			t.Errorf("%s: %v", src.File, err)
			continue
		}
		m := mars.New(cfg, 1)
		if err := m.Load(w, 1234); err != nil {
			t.Fatal(err)
		}
		for m.Cycle < cfg.MaxCycles && m.Alive() > 0 {
			m.Step()
		}
		if m.Alive() == 0 {
			t.Errorf("%s (%s) died alone at cycle %d", src.File, w.Name, m.Cycle)
		} else {
			t.Logf("%-22s %-20s %2d lines, %4d processes", src.File, w.Name, len(w.Code), m.Processes(0))
		}
	}
}

func TestLoadNames(t *testing.T) {
	ws, err := Load(mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range ws {
		if w.Name == "" {
			t.Errorf("warrior with no name")
		}
	}
}

func TestNameFallsBackToFileName(t *testing.T) {
	cfg := mars.DefaultConfig()
	for _, tt := range []struct{ file, src, want string }{
		{"nameless.red", "MOV 0, 1", "nameless"},
		{"commented.red", ";name Comment\nMOV 0, 1", "Comment"},
	} {
		w, err := Assemble(Source{File: tt.file, Code: tt.src}, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if w.Name != tt.want {
			t.Errorf("%s: name = %q, want %q", tt.file, w.Name, tt.want)
		}
	}
}

func TestLoadFS(t *testing.T) {
	fsys := fstest.MapFS{
		"a.red":           {Data: []byte(";name Alpha\nMOV 0, 1\n")},
		"sub/b.RED":       {Data: []byte("JMP 0\n")},
		"sub/deep/c.red":  {Data: []byte(";name Gamma\nSPL 0\n")},
		"sub/d.txt":       {Data: []byte("DAT 0, 0\n")},
		"sub/zork.red":    {Data: []byte(".name \"zork\"\nl2: sti r1, %:live, %1\n")},
		"README.md":       {Data: []byte("not a warrior\n")},
		".hidden":         {Data: []byte("garbage\n")},
		".git/config":     {Data: []byte("[core]\n")},
		"sub/empty/.keep": {Data: nil},
	}
	ws, err := LoadFS(fsys, mars.DefaultConfig())
	var names []string
	for _, w := range ws {
		names = append(names, w.Name)
	}
	if got, want := strings.Join(names, ","), "Alpha,b,Gamma"; got != want {
		t.Errorf("loaded %s, want %s", got, want)
	}
	if err == nil {
		t.Fatal("expected an error for sub/zork.red")
	}
	msg := err.Error()
	for _, want := range []string{"sub/zork.red"} {
		if !strings.Contains(msg, want) {
			t.Errorf("err = %v, want it to mention %s", err, want)
		}
	}
	for _, bad := range []string{".hidden", ".git", ".keep", "README.md", "d.txt"} {
		if strings.Contains(msg, bad) {
			t.Errorf("err = %v, hidden file %s should be skipped", err, bad)
		}
	}
}

func TestRoundRobin(t *testing.T) {
	cfg := mars.DefaultConfig()
	all := All()
	var ws []*mars.Warrior
	for _, src := range all {
		w, err := redcode.Assemble(src.Code, cfg)
		if err != nil {
			t.Fatal(err)
		}
		ws = append(ws, w)
	}
	m := mars.New(cfg, 2)
	for i := range ws {
		for j := range ws {
			if i >= j {
				continue
			}
			var wins [3]int
			for r := range 4 {
				m.Reset()
				m.Load(ws[i], 0)
				m.Load(ws[j], 2000+r*997)
				for {
					if done, winner := m.Done(); done {
						wins[winner+1]++
						break
					}
					m.Step()
				}
			}
			t.Logf("%-8s vs %-8s  %d-%d (%d ties)", ws[i].Name, ws[j].Name, wins[1], wins[2], wins[0])
		}
	}
}

// BenchmarkLoad loads and assembles the built-in roster from the embedded
// filesystem, as the demo does at start up.
func BenchmarkLoad(b *testing.B) {
	cfg := mars.DefaultConfig()
	b.ReportAllocs()
	for b.Loop() {
		Load(cfg)
	}
}

func TestRoster(t *testing.T) {
	r, err := NewRoster(FS, mars.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	all := All()
	if r.Len() != len(all) {
		t.Fatalf("Len = %d, want %d", r.Len(), len(all))
	}
	for i, src := range all {
		w, err := r.Warrior(i)
		want, wantErr := Assemble(src, mars.DefaultConfig())
		if (err != nil) != (wantErr != nil) {
			t.Errorf("%s: err = %v, want %v", src.File, err, wantErr)
			continue
		}
		if err == nil && (w.Name != want.Name || len(w.Code) != len(want.Code)) {
			t.Errorf("%s: got %s (%d), want %s (%d)", src.File, w.Name, len(w.Code), want.Name, len(want.Code))
		}
	}
}
