package archiver

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	for _, format := range []string{"zip", "7z", "tar.gz"} {
		for _, password := range []string{"", "synthetic-test"} {
			if format == "tar.gz" && password != "" {
				continue
			}
			t.Run(format+password, func(t *testing.T) {
				root := t.TempDir()
				src := filepath.Join(root, "hello.txt")
				os.WriteFile(src, []byte("synthetic test"), 0600)
				dest := filepath.Join(root, "result."+format)
				m := &Manager{}
				if e := m.work(context.Background(), []string{src}, dest, format, password, false); e != nil {
					t.Fatal(e)
				}
				files, e := List(context.Background(), dest, password)
				if e != nil || len(files) == 0 {
					t.Fatalf("%v %v", files, e)
				}

				out := filepath.Join(root, "extracted")
				if e = m.work(context.Background(), []string{dest}, out, "", password, true); e != nil {
					t.Fatal(e)
				}
				b, e := os.ReadFile(filepath.Join(out, "hello.txt"))
				if e != nil || string(b) != "synthetic test" {
					t.Fatalf("%s %v", b, e)
				}
				if e = m.work(context.Background(), []string{src}, dest, format, password, false); e == nil {
					t.Fatal("overwrote")
				}
			})
		}
	}
}
func TestUnsafe(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "bad.zip")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	w, _ := z.Create("../escape.txt")
	w.Write([]byte("bad"))
	z.Close()
	f.Close()
	m := &Manager{}
	if e := m.work(context.Background(), []string{p}, filepath.Join(root, "out"), "", "", true); e == nil {
		t.Fatal("unsafe extraction")
	}
}
func TestCancel(t *testing.T) {
	m := &Manager{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := m.work(ctx, []string{"missing"}, filepath.Join(t.TempDir(), "out.zip"), "zip", "", false); e == nil {
		t.Fatal("cancel ignored")
	}
	_ = time.Second
}
func TestMountedRoundTrip(t *testing.T) {
	base := os.Getenv("ZIPGALLERY_TEST_SHARE")
	if base == "" {
		t.Skip("explicit synthetic share test")
	}
	root, e := os.MkdirTemp(base, ".zipgallery-qa-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(root)
	src := filepath.Join(root, "synthetic.txt")
	os.WriteFile(src, []byte("synthetic SMB round trip"), 0600)
	m := &Manager{}
	dest := filepath.Join(root, "sample.7z")
	if e = m.work(context.Background(), []string{src}, dest, "7z", "synthetic-test", false); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, "unpacked")
	if e = m.work(context.Background(), []string{dest}, out, "", "synthetic-test", true); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(out, "synthetic.txt"))
	if e != nil || string(b) != "synthetic SMB round trip" {
		t.Fatal(e, string(b))
	}
}
func TestWrongPassword(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "text")
	os.WriteFile(src, []byte("sample"), 0600)
	m := &Manager{}
	dest := filepath.Join(root, "secret.7z")
	if e := m.work(context.Background(), []string{src}, dest, "7z", "correct", false); e != nil {
		t.Fatal(e)
	}
	if _, e := List(context.Background(), dest, "wrong"); e == nil || e.Error() != "E_PASSWORD" {
		t.Fatal(e)
	}
}
func TestRunningCancel(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "large.bin")
	f, e := os.Create(src)
	if e != nil {
		t.Fatal(e)
	}
	block := make([]byte, 1<<20)
	for i := range block {
		block[i] = byte(i*31 + i/251)
	}
	for i := 0; i < 128; i++ {
		f.Write(block)
	}
	f.Close()
	m := &Manager{}
	out := filepath.Join(root, "cancelled.7z")
	if _, e = m.Start([]string{src}, out, "7z", "", false); e != nil {
		t.Fatal(e)
	}
	time.Sleep(40 * time.Millisecond)
	m.Cancel()
	deadline := time.Now().Add(10 * time.Second)
	for m.Status().State == "running" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if m.Status().State != "cancelled" {
		t.Fatalf("%+v", m.Status())
	}
	if _, e = os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("partial output remains")
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".zipgallery-*"))
	if len(matches) > 0 {
		t.Fatal("temporary files remain")
	}
}
