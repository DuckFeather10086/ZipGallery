package archive

import (
	"archive/zip"
	"bytes"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRepairPreservesCompressedData(t *testing.T) {
	for _, codec := range []string{"gb18030", "shift-jis", "cp437"} {
		t.Run(codec, func(t *testing.T) {
			dir := t.TempDir()
			source, dest := filepath.Join(dir, "source.cbz"), filepath.Join(dir, "copy.cbz")
			enc, _ := legacyEncoding(codec)
			name := "café.jpg"
			if codec == "gb18030" {
				name = "图片/测试.jpg"
			}
			if codec == "shift-jis" {
				name = "日本語/画像.jpg"
			}
			raw, _ := enc.NewEncoder().String(name)
			var b bytes.Buffer
			z := zip.NewWriter(&b)
			w, _ := z.CreateHeader(&zip.FileHeader{Name: raw, NonUTF8: true, Method: zip.Deflate})
			w.Write(bytes.Repeat([]byte("synthetic image payload"), 100))
			z.Close()
			os.WriteFile(source, b.Bytes(), 0600)
			p, err := PreviewRepair(source, codec)
			if err != nil || len(p.Changes) != 1 || p.Changes[0].After != name {
				t.Fatal(p, err)
			}
			if err = RepairTo(source, dest, codec, p.Version); err != nil {
				t.Fatal(err)
			}
			out, err := zip.OpenReader(dest)
			if err != nil {
				t.Fatal(err)
			}
			defer out.Close()
			if out.File[0].Name != name || out.File[0].NonUTF8 || out.File[0].Flags&0x800 == 0 {
				t.Fatal(out.File[0].FileHeader)
			}
			in, _ := zip.OpenReader(source)
			defer in.Close()
			a, _ := in.File[0].OpenRaw()
			c, _ := out.File[0].OpenRaw()
			ab, _ := io.ReadAll(a)
			cb, _ := io.ReadAll(c)
			if !bytes.Equal(ab, cb) {
				t.Fatal("compressed data changed")
			}
			if err = RepairTo(source, dest, codec, p.Version); err == nil {
				t.Fatal("overwrote existing file")
			}
			if err = RepairTo(source, source, codec, p.Version); err == nil {
				t.Fatal("overwrote source")
			}
			original, _ := os.ReadFile(source)
			if !bytes.Equal(original, b.Bytes()) {
				t.Fatal("source changed")
			}
		})
	}
}
func TestWrongEncodingAndChangedSource(t *testing.T) {
	_, err := renameLegacy(string([]byte{0x81}), simplifiedchinese.GB18030)
	if err == nil {
		t.Fatal("invalid sequence accepted")
	}
	_, err = renameLegacy(string([]byte{0x81}), japanese.ShiftJIS)
	if err == nil {
		t.Fatal("invalid sjis accepted")
	}
}
