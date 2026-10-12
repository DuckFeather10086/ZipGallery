package gallery

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
	"zipgallery/internal/location"
)

func sample(t *testing.T) string {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, 800, 600))
	im.Set(10, 10, color.White)
	var jpg, b bytes.Buffer
	jpeg.Encode(&jpg, im, nil)
	z := zip.NewWriter(&b)
	for _, n := range []string{"1.jpg", "folder/2.jpg", "bad.jpg"} {
		w, _ := z.Create(n)
		if n == "bad.jpg" {
			w.Write([]byte("bad"))
		} else {
			w.Write(jpg.Bytes())
		}
	}
	z.Close()
	p := filepath.Join(t.TempDir(), "test.zip")
	os.WriteFile(p, b.Bytes(), 0600)
	return p
}
func TestImageProtocolProgressAndStaleSessions(t *testing.T) {
	state, cache := t.TempDir(), t.TempDir()
	s := New(state, cache)
	defer s.Close()
	p := sample(t)
	v, err := s.OpenLocation(location.Location{Path: p})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"thumb", "full"} {
		r := httptest.NewRecorder()
		s.ServeHTTP(r, httptest.NewRequest("GET", "gallery://localhost/"+kind+"/"+v.Entries[0].ID, nil))
		if r.Code != 200 {
			t.Fatal(r.Code, r.Body.String())
		}
		im, err := jpeg.Decode(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if kind == "thumb" && (im.Bounds().Dx() > 320 || im.Bounds().Dy() > 240) {
			t.Fatal("not a thumbnail")
		}
	}
	if err = s.Progress(v.ID, v.Entries[2].ID, "folder"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.OpenLocation(location.Location{Path: p + "-missing"}); err == nil {
		t.Fatal("missing accepted")
	}
	if s.Current().ID != v.ID {
		t.Fatal("failed open replaced session")
	}
	next, err := s.OpenLocation(location.Location{Path: p})
	if err != nil {
		t.Fatal(err)
	}
	if next.Progress != v.Entries[2].Identity {
		t.Fatal("lost progress")
	}
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest("GET", "gallery://localhost/full/"+v.Entries[0].ID, nil))
	if r.Code != 410 {
		t.Fatal("old session accessible")
	}
	r = httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest("GET", "gallery://localhost/full/"+next.Entries[1].ID, nil))
	if r.Code != 422 {
		t.Fatal("bad image accepted", r.Code)
	}
	if err = s.ClearCache(); err != nil {
		t.Fatal(err)
	}
	restored := New(state, t.TempDir())
	defer restored.Close()
	if restored.Recents()[0].Entry != next.Progress {
		t.Fatal("progress not persisted")
	}
}

func TestLegacyNamesRemainSerializable(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, _ := z.CreateHeader(&zip.FileHeader{Name: string([]byte{0xb2, 0xe2, 0xca, 0xd4}) + "/1.jpg", NonUTF8: true})
	w.Write([]byte("bad image"))
	z.Close()
	p := filepath.Join(t.TempDir(), "legacy.zip")
	os.WriteFile(p, b.Bytes(), 0600)
	s := New(t.TempDir(), t.TempDir())
	defer s.Close()
	v, err := s.OpenLocation(location.Location{Path: p})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range v.Entries {
		if !utf8.ValidString(e.Name) || !utf8.ValidString(e.Folder) || !utf8.ValidString(e.Identity) {
			t.Fatal("invalid UTF8 in public metadata")
		}
	}
}
