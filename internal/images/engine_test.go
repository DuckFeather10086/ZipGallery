package images

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"zipgallery/internal/archive"
	"zipgallery/internal/location"
)

func TestOrientationAndWebP(t *testing.T) {
	var b bytes.Buffer
	jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 40, 20)), nil)
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	jpg := append([]byte{0xff, 0xd8, 0xff, 0xe1, 0, byte(len(exif) + 2)}, exif...)
	jpg = append(jpg, b.Bytes()[2:]...)
	webp, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	var archiveBytes bytes.Buffer
	z := zip.NewWriter(&archiveBytes)
	for n, data := range map[string][]byte{"oriented.jpg": jpg, "one.webp": webp} {
		w, _ := z.Create(n)
		w.Write(data)
	}
	z.Close()
	p := filepath.Join(t.TempDir(), "images.zip")
	os.WriteFile(p, archiveBytes.Bytes(), 0600)
	s, err := archive.Open(location.Location{Path: p})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e := New(t.TempDir())
	for _, a := range s.Entries {
		out, err := e.Get(context.Background(), s, a, false, 0)
		if err != nil {
			t.Fatal(a.Name, err)
		}
		img, err := jpeg.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatal(err)
		}
		if a.Name == "oriented.jpg" && (img.Bounds().Dx() != 20 || img.Bounds().Dy() != 40) {
			t.Fatal("EXIF orientation not applied", img.Bounds())
		}
		if a.Name == "one.webp" && (img.Bounds().Dx() != 1 || img.Bounds().Dy() != 1) {
			t.Fatal("webp dimensions", img.Bounds())
		}
	}
}
