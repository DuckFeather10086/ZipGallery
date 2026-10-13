package gallery

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"zipgallery/internal/archiver"
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

func TestEncryptedPreviewAndHistory(t *testing.T) {
	for _, variant := range []string{"zip", "7z", "7z-headers"} {
		t.Run(variant, func(t *testing.T) {
			format := strings.TrimSuffix(variant, "-headers")
			root := t.TempDir()
			src := filepath.Join(root, "pages")
			if err := os.MkdirAll(filepath.Join(src, "chapter"), 0700); err != nil {
				t.Fatal(err)
			}
			im := image.NewRGBA(image.Rect(0, 0, 32, 32))
			var jpg bytes.Buffer
			if err := jpeg.Encode(&jpg, im, nil); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(src, "chapter", "1.jpg"), jpg.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(root, "secret."+format)
			args := []string{"a", "-t" + format, "-p"}
			if variant == "7z-headers" {
				args = append(args, "-mhe=on")
			}
			args = append(args, p, "pages")
			cmd := exec.Command(archiver.Binary(), args...)
			cmd.Dir = root
			cmd.Stdin = strings.NewReader("correct\ncorrect\n")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			cache := t.TempDir()
			s := New(t.TempDir(), cache)
			defer s.Close()
			plain := sample(t)
			before, err := s.OpenLocation(location.Location{Path: plain})
			if err != nil {
				t.Fatal(err)
			}
			for _, password := range []string{"", "wrong"} {
				if _, err := s.OpenArchive(location.Location{Path: p}, password); err == nil || err.Error() != "E_PASSWORD" {
					t.Fatalf("password %q: %v", password, err)
				}
				if s.Current().ID != before.ID {
					t.Fatal("failed unlock replaced previous view")
				}
				if s.ArchivePassword(p) != "" {
					t.Fatal("unverified password remembered")
				}
			}
			v, err := s.OpenArchive(location.Location{Path: p}, "correct")
			if err != nil {
				t.Fatal(err)
			}
			if len(v.Entries) != 1 || v.Entries[0].Name != "pages/chapter/1.jpg" || v.Entries[0].Folder != "pages/chapter" || v.Location.Path != p {
				t.Fatalf("bad preview: %+v", v)
			}
			session := s.session
			for _, kind := range []string{"thumb", "full"} {
				r := httptest.NewRecorder()
				s.ServeHTTP(r, httptest.NewRequest("GET", "gallery://localhost/"+kind+"/"+v.Entries[0].ID, nil))
				if r.Code != 200 {
					t.Fatal(r.Code, r.Body.String())
				}
			}
			disk, err := os.ReadDir(cache)
			if err != nil || len(disk) != 0 {
				t.Fatalf("encrypted preview persisted to thumbnail cache: %v %v", disk, err)
			}
			if err := s.Progress(v.ID, v.Entries[0].ID, v.Entries[0].Folder); err != nil {
				t.Fatal(err)
			}
			if _, err := s.OpenLocation(location.Location{Path: plain}); err != nil {
				t.Fatal(err)
			}
			if _, err := session.Read(v.Entries[0]); err == nil {
				t.Fatal("closed preview still accessible")
			}
			restored, err := s.OpenLocation(location.Location{Path: p})
			if err != nil {
				t.Fatal(err)
			}
			if restored.Progress != v.Entries[0].Identity || restored.Folder != v.Entries[0].Folder {
				t.Fatalf("lost history: %+v", restored)
			}
			if restored.Entries[0].ID == v.Entries[0].ID {
				t.Fatal("stale entry ID reused")
			}
			out := filepath.Join(root, "extracted")
			if err := archiver.ExtractTo(context.Background(), p, out, s.ArchivePassword(p)); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(out, "pages", "chapter", "1.jpg"))
			if err != nil || !bytes.Equal(b, jpg.Bytes()) {
				t.Fatal("extraction failed", err)
			}
			// Modifying the source invalidates the remembered password.
			future := time.Now().Add(time.Second * 2)
			if err := os.Chtimes(p, future, future); err != nil {
				t.Fatal(err)
			}
			if s.ArchivePassword(p) != "" {
				t.Fatal("password reused for changed source")
			}
			if _, err := s.OpenLocation(location.Location{Path: p}); err == nil || err.Error() != "E_PASSWORD" {
				t.Fatal("changed archive did not relock", err)
			}
			s.Close()
			if s.Current().ID != "" || s.ArchivePassword(p) != "" {
				t.Fatal("close retained view or password")
			}
		})
	}
}
