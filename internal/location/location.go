// Package location resolves sources without coupling archive readers to the UI.
package location

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Location struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}
type Source struct {
	io.ReaderAt
	io.Closer
	Size     int64
	Version  string
	Location Location
}

func Open(l Location) (*Source, error) {
	if l.Kind != "" && l.Kind != "local" && l.Kind != "mounted/local" {
		return nil, fmt.Errorf("E_SOURCE: %s", l.Kind)
	}
	p, err := filepath.Abs(l.Path)
	if err != nil {
		return nil, err
	}
	p = filepath.Clean(p)
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("E_UNREACHABLE: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !st.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("E_FILE")
	}
	l.Path = p
	l.Kind = "local"
	if strings.HasPrefix(p, "/Volumes/") || isMountedSMB(p) {
		l.Kind = "mounted/local"
	}
	return &Source{f, f, st.Size(), fmt.Sprintf("%d-%d", st.Size(), st.ModTime().UnixNano()), l}, nil
}
