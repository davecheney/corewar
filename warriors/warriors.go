// Package warriors holds the built-in warriors the demo picks from, and
// loads others from any fs.FS. Each warrior is a .red file of ICWS'94
// Redcode.
package warriors

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/davecheney/corewar/mars"
	"github.com/davecheney/corewar/redcode"
)

// FS holds the Redcode source of every built-in warrior as *.red files.
//
//go:embed *.red
var FS embed.FS

// Source is a warrior's Redcode source and the file it came from.
type Source struct {
	File string
	Code string
}

// All returns every built-in warrior's source, sorted by file name.
func All() []Source {
	srcs, err := Sources(FS)
	if err != nil {
		panic(err)
	}
	return srcs
}

// Sources returns the source of every .red file in fsys, walking
// subdirectories, in lexical order of path. Other files, and hidden files
// and directories whose names start with a dot, are skipped.
func Sources(fsys fs.FS) ([]Source, error) {
	files, err := Files(fsys)
	if err != nil {
		return nil, err
	}
	out := make([]Source, 0, len(files))
	for _, p := range files {
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		out = append(out, Source{File: p, Code: string(b)})
	}
	return out, nil
}

// Files returns the path of every .red file in fsys, as Sources does,
// without reading them.
func Files(fsys fs.FS) ([]string, error) {
	var out []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != "." && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !IsRedcodeFile(p) {
			return nil
		}
		out = append(out, p)
		return nil
	})
	return out, err
}

// Roster reads and assembles the .red files in a file system one at a
// time, on demand, so that only the warriors in use need be held in
// memory. It satisfies ui.Roster.
type Roster struct {
	fsys  fs.FS
	files []string
	cfg   mars.Config
}

// NewRoster lists the .red files in fsys, as Files does, for battles under
// cfg. Nothing is read or assembled until Warrior is called.
func NewRoster(fsys fs.FS, cfg mars.Config) (*Roster, error) {
	files, err := Files(fsys)
	if err != nil {
		return nil, err
	}
	return &Roster{fsys: fsys, files: files, cfg: cfg}, nil
}

// Len returns the number of warriors in the roster.
func (r *Roster) Len() int { return len(r.files) }

// Warrior reads and assembles the i'th warrior.
func (r *Roster) Warrior(i int) (*mars.Warrior, error) {
	b, err := fs.ReadFile(r.fsys, r.files[i])
	if err != nil {
		return nil, err
	}
	return Assemble(Source{File: r.files[i], Code: string(b)}, r.cfg)
}

// IsRedcodeFile reports whether name has a .red extension, in any case.
func IsRedcodeFile(name string) bool {
	return strings.EqualFold(path.Ext(name), ".red")
}

// Load assembles every built-in warrior for a battle under cfg.
func Load(cfg mars.Config) ([]*mars.Warrior, error) {
	return LoadFS(FS, cfg)
}

// LoadFS assembles every .red file in fsys, walking subdirectories.
//
// A file that fails to assemble does not stop the others loading: LoadFS
// returns every warrior that did assemble along with an error joining one
// entry per file that did not.
func LoadFS(fsys fs.FS, cfg mars.Config) ([]*mars.Warrior, error) {
	srcs, err := Sources(fsys)
	if err != nil {
		return nil, err
	}
	var ws []*mars.Warrior
	var errs []error
	for _, src := range srcs {
		w, err := Assemble(src, cfg)
		if err != nil {
			errs = append(errs, err)
		}
		if w != nil {
			ws = append(ws, w)
		}
	}
	return ws, errors.Join(errs...)
}

// Assemble assembles src for a battle under cfg. A warrior with no ;name
// comment is named after its file.
func Assemble(src Source, cfg mars.Config) (*mars.Warrior, error) {
	w, err := redcode.Assemble(src.Code, cfg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", src.File, err)
	}
	if w.Name == "" {
		base := path.Base(src.File)
		w.Name = strings.TrimSuffix(base, path.Ext(base))
	}
	return w, nil
}
