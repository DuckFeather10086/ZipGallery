package gallery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"zipgallery/internal/archive"
	"zipgallery/internal/archiver"
	"zipgallery/internal/images"
	"zipgallery/internal/location"
)

type Recent struct {
	Location location.Location `json:"location"`
	Entry    string            `json:"entry"`
	Folder   string            `json:"folder"`
	Updated  int64             `json:"updated"`
}
type View struct {
	Files    []archiver.File   `json:"files"`
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Location location.Location `json:"location"`
	Entries  []archive.Entry   `json:"entries"`
	Progress string            `json:"progress"`
	Folder   string            `json:"folder"`
}
type Service struct {
	mu        sync.Mutex
	openMu    sync.Mutex
	session   *archive.Session
	general   *View
	entries   map[string]archive.Entry
	recents   []Recent
	passwords map[string]credential
	stateFile string
	Engine    *images.Engine
}

type credential struct{ password, version string }

func New(state, cache string) *Service {
	s := &Service{stateFile: filepath.Join(state, "recent.json"), Engine: images.New(cache), recents: []Recent{}, passwords: map[string]credential{}}
	_ = os.MkdirAll(state, 0700)
	b, err := os.ReadFile(s.stateFile)
	if err == nil {
		_ = json.Unmarshal(b, &s.recents)
	}
	return s
}
func (s *Service) OpenLocation(l location.Location) (View, error) {
	return s.OpenArchive(l, "")
}
func (s *Service) OpenArchive(l location.Location, password string) (View, error) {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	// Resolve the original source before listing; history always keeps this path.
	src, err := location.Open(l)
	if err != nil {
		return View{}, err
	}
	l = src.Location
	version := src.Version
	src.Close()
	if strings.ContainsAny(password, "\r\n") {
		return View{}, fmt.Errorf("E_PASSWORD")
	}
	if password == "" {
		password = s.ArchivePassword(l.Path)
	}
	files, listErr := archiver.List(context.Background(), l.Path, password)
	if listErr != nil {
		return View{}, listErr
	}
	encrypted := false
	for _, f := range files {
		encrypted = encrypted || f.Encrypted
	}
	var next *archive.Session
	if encrypted {
		if password == "" {
			return View{}, fmt.Errorf("E_PASSWORD")
		}
		root, e := os.MkdirTemp("", "zipgallery-preview-")
		if e != nil {
			return View{}, e
		}
		output := filepath.Join(root, "files")
		if e = archiver.ExtractTo(context.Background(), l.Path, output, password); e != nil {
			os.RemoveAll(root)
			return View{}, e
		}
		// The session owns the outer directory so closing it removes all plaintext.
		next, err = archive.OpenExtracted(l, root, output)
	} else {
		next, err = archive.Open(l)
	}
	if err != nil && encrypted && err.Error() != "E_NO_IMAGES" {
		return View{}, err
	}
	if err != nil {
		s.mu.Lock()
		old := s.session
		s.session = nil
		s.entries = nil
		v := View{ID: fmt.Sprint(time.Now().UnixNano()), Name: filepath.Base(l.Path), Location: l, Entries: []archive.Entry{}, Files: files}
		s.general = &v
		if password != "" {
			s.passwords[l.Path] = credential{password, version}
		}
		s.upsert(Recent{Location: l, Updated: time.Now().Unix()})
		s.mu.Unlock()
		if old != nil {
			old.Close()
		}
		return v, nil
	}
	s.mu.Lock()
	s.general = nil
	if password != "" {
		s.passwords[l.Path] = credential{password, version}
	}
	old := s.session
	s.session = next
	s.entries = map[string]archive.Entry{}
	for _, e := range next.Entries {
		s.entries[e.ID] = e
	}
	r := Recent{Location: next.Source.Location, Updated: time.Now().Unix()}
	for _, x := range s.recents {
		if x.Location.Path == r.Location.Path {
			r.Entry = x.Entry
			r.Folder = x.Folder
			break
		}
	}
	s.upsert(r)
	v := s.view()
	v.Files = files
	s.general = &v
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return v, nil
}

// ArchivePassword returns only credentials verified for this exact source version.
// It is used internally for extraction and is never exposed through the UI bridge.
func (s *Service) ArchivePassword(path string) string {
	src, err := location.Open(location.Location{Path: path})
	if err != nil {
		return ""
	}
	defer src.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.passwords[src.Location.Path]
	if c.version == src.Version {
		return c.password
	}
	return ""
}
func (s *Service) view() View {
	if s.general != nil {
		v := *s.general
		for _, r := range s.recents {
			if r.Location.Path == v.Location.Path {
				v.Progress = r.Entry
				v.Folder = r.Folder
			}
		}
		return v
	}
	if s.session == nil {
		return View{Entries: []archive.Entry{}, Files: []archiver.File{}}
	}
	x := s.session
	v := View{ID: x.ID, Name: filepath.Base(x.Source.Location.Path), Location: x.Source.Location, Entries: x.Entries}
	for _, r := range s.recents {
		if r.Location.Path == v.Location.Path {
			v.Progress = r.Entry
			v.Folder = r.Folder
		}
	}
	return v
}
func (s *Service) Current() View { s.mu.Lock(); defer s.mu.Unlock(); return s.view() }
func (s *Service) Recents() []Recent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Recent{}, s.recents...)
}
func (s *Service) upsert(r Recent) {
	out := []Recent{r}
	for _, x := range s.recents {
		if x.Location.Path != r.Location.Path && len(out) < 20 {
			out = append(out, x)
		}
	}
	s.recents = out
	_ = s.save()
}
func (s *Service) save() error {
	b, err := json.MarshalIndent(s.recents, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.stateFile + ".tmp"
	if err = os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.stateFile)
}
func (s *Service) Progress(session, id, folder string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil || s.session.ID != session {
		return nil
	}
	entry := ""
	if id != "" {
		a, ok := s.entries[id]
		if !ok {
			return fmt.Errorf("E_SESSION")
		}
		entry = a.Identity
	}
	for i, r := range s.recents {
		if r.Location.Path == s.session.Source.Location.Path {
			if entry != "" {
				r.Entry = entry
			}
			r.Folder = folder
			r.Updated = time.Now().Unix()
			s.recents[i] = r
			break
		}
	}
	return s.save()
}
func (s *Service) ClearCache() error { return s.Engine.Clear() }
func (s *Service) Close() {
	s.openMu.Lock()
	defer s.openMu.Unlock()
	s.mu.Lock()
	old := s.session
	s.session = nil
	s.general = nil
	s.passwords = map[string]credential{}
	s.entries = nil
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 2 || (parts[0] != "thumb" && parts[0] != "full") {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	session := s.session
	a, ok := s.entries[parts[1]]
	s.mu.Unlock()
	if !ok || session == nil {
		http.Error(w, "E_SESSION", 410)
		return
	}
	priority := 0
	if parts[0] == "thumb" {
		priority = 1
	}
	if r.URL.Query().Get("prefetch") == "1" {
		priority = 2
	}
	b, err := s.Engine.Get(r.Context(), session, a, parts[0] == "thumb", priority)
	if err != nil {
		http.Error(w, err.Error(), 422)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(b)
}
