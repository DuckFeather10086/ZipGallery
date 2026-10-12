package archive

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"hash/crc32"
	"io"
	"path"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
	"zipgallery/internal/location"
)

const MaxBytes = 96 << 20

type Entry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Folder   string `json:"folder"`
	Identity string `json:"identity"`
	Index    int    `json:"-"`
}
type Session struct {
	ID      string
	Source  *location.Source
	Entries []Entry
	files   []*zip.File
	mu      sync.RWMutex
	closed  bool
	Context context.Context
	cancel  context.CancelFunc
}

func Open(l location.Location) (*Session, error) {
	src, err := location.Open(l)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(src, src.Size)
	if err != nil {
		src.Close()
		return nil, fmt.Errorf("E_BAD_ZIP: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{ID: fmt.Sprintf("%x", rand.Text()), Source: src, files: zr.File, Context: ctx, cancel: cancel}
	counts := map[string]int{}
	for i, f := range zr.File {
		n := f.Name
		parts := strings.Split(n, "/")
		skip := f.FileInfo().IsDir()
		for _, p := range parts {
			if p == "__MACOSX" || strings.HasPrefix(p, "._") {
				skip = true
			}
		}
		if skip {
			continue
		}
		ext := strings.ToLower(path.Ext(n))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}
		counts[n]++
		folder := path.Dir(n)
		if folder == "." {
			folder = ""
		}
		identity := fmt.Sprintf("%s#%d", n, counts[n])
		if !utf8.ValidString(n) {
			identity = fmt.Sprintf("legacy:%x#%d", sha256.Sum256([]byte(n)), counts[n])
		}
		s.Entries = append(s.Entries, Entry{fmt.Sprintf("%s-%d", s.ID, i), strings.ToValidUTF8(n, "�"), strings.ToValidUTF8(folder, "�"), identity, i})
	}
	sort.SliceStable(s.Entries, func(i, j int) bool { return NaturalLess(s.Entries[i].Name, s.Entries[j].Name) })
	if len(s.Entries) == 0 {
		s.Close()
		return nil, fmt.Errorf("E_NO_IMAGES")
	}
	return s, nil
}
func NaturalLess(a, b string) bool {
	aa, bb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	i, j := 0, 0
	for i < len(aa) && j < len(bb) {
		if aa[i] >= '0' && aa[i] <= '9' && bb[j] >= '0' && bb[j] <= '9' {
			x, y := i, j
			for i < len(aa) && aa[i] >= '0' && aa[i] <= '9' {
				i++
			}
			for j < len(bb) && bb[j] >= '0' && bb[j] <= '9' {
				j++
			}
			as, bs := strings.TrimLeft(string(aa[x:i]), "0"), strings.TrimLeft(string(bb[y:j]), "0")
			if len(as) != len(bs) {
				return len(as) < len(bs)
			}
			if as != bs {
				return as < bs
			}
		} else {
			ar, br := unicode.ToLower(aa[i]), unicode.ToLower(bb[j])
			if ar != br {
				return ar < br
			}
			i++
			j++
		}
	}
	if i != len(aa) || j != len(bb) {
		return i == len(aa)
	}
	return a < b
}
func (s *Session) Read(e Entry) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fmt.Errorf("E_SESSION")
	}
	if err := s.Context.Err(); err != nil {
		return nil, err
	}
	f := s.files[e.Index]
	if f.Flags&1 != 0 {
		return nil, fmt.Errorf("E_ENCRYPTED")
	}
	if f.Method != zip.Store && f.Method != zip.Deflate {
		return nil, fmt.Errorf("E_METHOD: %d", f.Method)
	}
	if f.UncompressedSize64 > MaxBytes {
		return nil, fmt.Errorf("E_BYTES")
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if len(b) > MaxBytes {
		return nil, fmt.Errorf("E_BYTES")
	}
	if err != nil {
		return nil, fmt.Errorf("E_READ: %w", err)
	}
	if crc32.ChecksumIEEE(b) != f.CRC32 {
		return nil, fmt.Errorf("E_READ: CRC32 mismatch")
	}
	return b, nil
}
func (s *Session) Close() {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.Source.Close()
	}
}
