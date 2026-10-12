package archiver

import (
	"context"
	"github.com/ebitengine/purego"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Use the system call when available; SMB servers may not implement RENAME_EXCL.
func publish(ctx context.Context, source, dest string) error {
	lib, e := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW)
	if e == nil {
		defer purego.Dlclose(lib)
		var rename func(string, string, uint32) int32
		purego.RegisterLibFunc(&rename, lib, "renamex_np")
		if rename(source, dest, 4) == 0 {
			return nil
		}
	}
	if _, e = os.Lstat(dest); !os.IsNotExist(e) {
		return syscall.EEXIST
	}
	info, e := os.Lstat(source)
	if e != nil {
		return e
	}
	if info.IsDir() {
		if e = os.Mkdir(dest, 0700); e != nil {
			return e
		}
		e = copyTree(ctx, source, dest)
		if e != nil {
			os.RemoveAll(dest)
		}
		return e
	}
	return copyFile(ctx, source, dest)
}
func copyTree(ctx context.Context, source, dest string) error {
	return filepath.WalkDir(source, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if p == source {
			return nil
		}
		rel, e := filepath.Rel(source, p)
		if e != nil {
			return e
		}
		target := filepath.Join(dest, rel)
		if d.Type()&os.ModeSymlink != 0 {
			return syscall.EINVAL
		}
		if d.IsDir() {
			return os.Mkdir(target, 0700)
		}
		return copyFile(ctx, p, target)
	})
}

type cancelReader struct {
	ctx context.Context
	r   io.Reader
}

func (r cancelReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
func copyFile(ctx context.Context, source, dest string) error {
	in, e := os.Open(source)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, cancelReader{ctx, in})
	ce := out.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		os.Remove(dest)
	}
	return e
}
