package warriors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davecheney/corewar/mars"
)

func TestLoadPaths(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) string {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	write("hill/a.red", ";name Alpha\nMOV 0, 1\n")
	write("hill/nested/b.red", "JMP 0\n")
	bad := write("hill/nested/zork.red", ".name \"zork\"\n")
	write("hill/notes.txt", "MOV 0, 1\n")
	write("hill/.DS_Store", "\x00\x01binary")
	single := write("single.txt", ";name Single\nSPL 0\n")

	ws, err := LoadPaths([]string{
		filepath.Join(dir, "hill"),
		single,
		filepath.Join(dir, "missing"),
	}, mars.DefaultConfig())

	var names []string
	for _, w := range ws {
		names = append(names, w.Name)
	}
	if got, want := strings.Join(names, ","), "Alpha,b,Single"; got != want {
		t.Errorf("loaded %s, want %s", got, want)
	}
	if err == nil {
		t.Fatal("expected errors for zork.red and the missing path")
	}
	for _, want := range []string{bad, filepath.Join(dir, "missing")} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to mention %s", err, want)
		}
	}
}
