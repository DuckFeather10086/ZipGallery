package archive

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"zipgallery/internal/location"
)

func TestArchiveIndexAndLifecycle(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for _, n := range []string{"10.jpg", "2.jpg", "1.jpg", "日本語/1.jpg", "other/1.jpg", "1.jpg", "__MACOSX/x.jpg", "a/._x.jpg"} {
		w, _ := z.Create(n)
		w.Write([]byte(n))
	}
	z.Close()
	p := filepath.Join(t.TempDir(), "sample.cbz")
	os.WriteFile(p, b.Bytes(), 0600)
	s, err := Open(location.Location{Path: p})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Entries) != 6 {
		t.Fatalf("entries: %v", s.Entries)
	}
	for i, want := range []string{"1.jpg", "1.jpg", "2.jpg", "10.jpg"} {
		if s.Entries[i].Name != want {
			t.Fatal(s.Entries)
		}
	}
	if s.Entries[0].ID == s.Entries[1].ID || s.Entries[0].Identity == s.Entries[1].Identity {
		t.Fatal("duplicate names collided")
	}
	s.Close()
	if _, err = s.Read(s.Entries[0]); err == nil {
		t.Fatal("read after close")
	}
}
func TestNaturalOrder(t *testing.T) {
	for _, p := range [][2]string{{"1.jpg", "2.jpg"}, {"2.jpg", "10.jpg"}, {"01.jpg", "1.jpg"}, {"x9/a", "x10/a"}, {"a9999999999999999999", "a10000000000000000000"}} {
		if !NaturalLess(p[0], p[1]) || NaturalLess(p[1], p[0]) {
			t.Fatal(p)
		}
	}
}

func TestUnsupportedAndDamagedInputs(t *testing.T) {
	for _, mode := range []string{"method", "encrypted", "huge", "crc"} {
		t.Run(mode, func(t *testing.T) {
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			h := &zip.FileHeader{Name: "bad.jpg", Method: zip.Store}
			switch mode {
			case "method":
				h.Method = 99
			case "encrypted":
				h.Flags = 1
			case "huge":
				h.UncompressedSize64 = MaxBytes + 1
			case "crc":
				h.UncompressedSize64 = 4
				h.CompressedSize64 = 4
				h.CRC32 = 0
			}
			w, err := z.CreateRaw(h)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "crc" {
				w.Write([]byte("data"))
			}
			z.Close()
			p := filepath.Join(t.TempDir(), "bad.zip")
			os.WriteFile(p, b.Bytes(), 0600)
			s, err := Open(location.Location{Path: p})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if _, err = s.Read(s.Entries[0]); err == nil {
				t.Fatal("unsupported or damaged input accepted")
			}
		})
	}
}

func TestExtractedPreviewCleanup(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.zip")
	if err := os.WriteFile(source, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, images := range []bool{true, false} {
		root := filepath.Join(t.TempDir(), "preview")
		content := filepath.Join(root, "files", "chapter")
		if err := os.MkdirAll(content, 0700); err != nil {
			t.Fatal(err)
		}
		if images {
			if err := os.WriteFile(filepath.Join(content, "1.jpg"), []byte("image bytes"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		s, err := OpenExtracted(location.Location{Path: source}, root, filepath.Join(root, "files"))
		if images {
			if err != nil {
				t.Fatal(err)
			}
			if s.Entries[0].Name != "chapter/1.jpg" || s.Source.Location.Path != source {
				t.Fatalf("bad source mapping: %+v", s.Entries)
			}
			b, err := s.Read(s.Entries[0])
			if err != nil || string(b) != "image bytes" {
				t.Fatal("cannot read preview", err)
			}
			s.Close()
			s.Close()
		} else if err == nil || err.Error() != "E_NO_IMAGES" {
			t.Fatal("empty preview accepted", err)
		}
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatal("plaintext temporary directory remains", err)
		}
	}
}
