package warriors

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davecheney/corewar/mars"
)

// LoadPaths assembles warriors from the host filesystem. Each path may be a
// directory, which is walked for .red files as LoadFS does, or a single
// file, which is loaded whatever its extension.
//
// As with LoadFS, files that fail to assemble are skipped: LoadPaths returns
// every warrior that did assemble along with an error joining one entry per
// path or file that did not. Errors name files by their path on disk.
func LoadPaths(paths []string, cfg mars.Config) ([]*mars.Warrior, error) {
	var ws []*mars.Warrior
	var errs []error
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var srcs []Source
		if info.IsDir() {
			srcs, err = Sources(os.DirFS(p))
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", p, err))
				continue
			}
			for i := range srcs {
				srcs[i].File = filepath.Join(p, filepath.FromSlash(srcs[i].File))
			}
		} else {
			b, err := os.ReadFile(p)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			srcs = []Source{{File: p, Code: string(b)}}
		}
		for _, src := range srcs {
			w, err := Assemble(src, cfg)
			if err != nil {
				errs = append(errs, err)
			}
			if w != nil {
				ws = append(ws, w)
			}
		}
	}
	return ws, errors.Join(errs...)
}
