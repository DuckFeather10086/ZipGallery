package archiver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type File struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Folder    bool   `json:"folder"`
	Encrypted bool   `json:"encrypted"`
}
type Job struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Percent int    `json:"percent"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}
type Manager struct {
	mu     sync.Mutex
	job    Job
	cancel context.CancelFunc
}

func Binary() string {
	exe, _ := os.Executable()
	p := filepath.Join(filepath.Dir(exe), "../Resources/7zip/7zz")
	if _, e := os.Stat(p); e == nil {
		return p
	}
	cwd, _ := os.Getwd()
	for {
		p = filepath.Join(cwd, "third_party/7zip/7zz")
		if _, e := os.Stat(p); e == nil {
			return p
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return p
}

type capture struct {
	mu       sync.Mutex
	b        bytes.Buffer
	progress func(int)
}

var percent = regexp.MustCompile(`(\d{1,3})%`)

func (w *capture) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.b.Len() < 32<<20 {
		w.b.Write(p)
	}
	if w.progress != nil {
		for _, m := range percent.FindAllSubmatch(p, -1) {
			n, _ := strconv.Atoi(string(m[1]))
			w.progress(n)
		}
	}
	return len(p), nil
}
func run(ctx context.Context, args []string, password, dir string, progress func(int), writeRoot string) (string, error) {
	cmd := exec.CommandContext(ctx, Binary(), args...)
	cmd.Dir = dir
	// Passwords are delivered on stdin, never included in process arguments.
	cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	if writeRoot != "" {
		root, _ := filepath.EvalSymlinks(writeRoot)
		profile := fmt.Sprintf(`(version 1)(allow default)(deny file-write*)(allow file-write* (subpath %q))`, root)
		cmd = exec.CommandContext(ctx, "/usr/bin/sandbox-exec", append([]string{"-p", profile, Binary()}, args...)...)
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(password + "\n" + password + "\n")
	}
	out := &capture{progress: progress}
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	s := out.b.String()
	if ctx.Err() != nil {
		return s, fmt.Errorf("E_CANCELLED")
	}
	if err != nil {
		if strings.Contains(strings.ToLower(s), "password") || strings.Contains(s, "Headers Error") {
			return s, fmt.Errorf("E_PASSWORD")
		}
		return s, fmt.Errorf("E_ARCHIVER")
	}
	return s, nil
}
func wrappedTar(path string) bool {
	p := strings.ToLower(path)
	return strings.HasSuffix(p, ".tar.gz") || strings.HasSuffix(p, ".tgz") || strings.HasSuffix(p, ".tar.xz") || strings.HasSuffix(p, ".tar.bz2")
}
func List(ctx context.Context, path, password string) ([]File, error) {
	if wrappedTar(path) {
		tmp, e := os.MkdirTemp("", "zipgallery-list-")
		if e != nil {
			return nil, fmt.Errorf("E_OUTPUT")
		}
		defer os.RemoveAll(tmp)
		_, e = run(ctx, []string{"x", "-y", "-o" + tmp, "--", path}, password, "", nil, tmp)
		if e != nil {
			return nil, e
		}
		items, e := os.ReadDir(tmp)
		if e != nil || len(items) != 1 || !items[0].Type().IsRegular() {
			return nil, fmt.Errorf("E_ARCHIVER")
		}
		return List(ctx, filepath.Join(tmp, items[0].Name()), password)
	}

	text, err := run(ctx, []string{"l", "-slt", "-ba", "-sccUTF-8", "--", path}, password, "", nil, "")
	if err != nil {
		return nil, err
	}
	files := []File{}
	var f *File
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "Path = ") {
			if f != nil {
				files = append(files, *f)
			}
			f = &File{Path: strings.TrimPrefix(line, "Path = ")}
		} else if f != nil {
			switch {
			case strings.HasPrefix(line, "Size = "):
				f.Size, _ = strconv.ParseInt(strings.TrimPrefix(line, "Size = "), 10, 64)
			case line == "Folder = +" || strings.HasPrefix(line, "Attributes = D"):
				f.Folder = true
			case line == "Encrypted = +":
				f.Encrypted = true
			case (strings.HasPrefix(line, "Symbolic Link = ") && strings.TrimPrefix(line, "Symbolic Link = ") != "") || (strings.HasPrefix(line, "Hard Link = ") && strings.TrimPrefix(line, "Hard Link = ") != ""):
				return nil, fmt.Errorf("E_UNSAFE_ARCHIVE")
			}
		}
	}
	if f != nil {
		files = append(files, *f)
	}
	return files, nil
}
func (m *Manager) Status() Job { m.mu.Lock(); defer m.mu.Unlock(); return m.job }
func (m *Manager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		m.cancel()
	}
}
func (m *Manager) Start(sources []string, dest, format, password string, extract bool) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job.State == "running" {
		return Job{}, fmt.Errorf("E_BUSY")
	}
	if strings.ContainsAny(password, "\r\n") {
		return Job{}, fmt.Errorf("E_PASSWORD")
	}
	if _, err := os.Lstat(dest); !os.IsNotExist(err) {
		return Job{}, fmt.Errorf("E_EXISTS")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.job = Job{ID: fmt.Sprint(time.Now().UnixNano()), State: "running", Output: dest}
	j := m.job
	go func() {
		err := m.work(ctx, sources, dest, format, password, extract)
		m.mu.Lock()
		defer m.mu.Unlock()
		m.cancel = nil
		cancel()
		if err != nil {
			m.job.State = "failed"
			m.job.Error = err.Error()
			if ctx.Err() != nil && err.Error() == "E_CANCELLED" {
				m.job.State = "cancelled"
			}
		} else {
			m.job.State = "succeeded"
			m.job.Percent = 100
		}
	}()
	return j, nil
}
func (m *Manager) work(ctx context.Context, sources []string, dest, format, password string, extract bool) error {
	parent := filepath.Dir(dest)
	tmp, err := os.MkdirTemp(parent, ".zipgallery-")
	if err != nil {
		return fmt.Errorf("E_OUTPUT")
	}
	defer os.RemoveAll(tmp)
	progress := func(n int) { m.mu.Lock(); m.job.Percent = min(n, 99); m.mu.Unlock() }
	output := filepath.Join(tmp, "output")
	if extract {
		files, e := List(ctx, sources[0], password)
		if e != nil {
			return e
		}
		for _, f := range files {
			p := strings.ReplaceAll(f.Path, "\\", "/")
			if strings.HasPrefix(p, "/") || strings.Contains(p, ":") {
				return fmt.Errorf("E_UNSAFE_ARCHIVE")
			}
			for _, part := range strings.Split(p, "/") {
				if part == ".." {
					return fmt.Errorf("E_UNSAFE_ARCHIVE")
				}
			}
		}
		if e = os.Mkdir(output, 0700); e != nil {
			return e
		}
		_, err = run(ctx, []string{"x", "-y", "-bsp1", "-bso1", "-o" + output, "--", sources[0]}, password, "", progress, tmp)

		if err == nil && wrappedTar(sources[0]) {
			children, e := os.ReadDir(output)
			if e != nil || len(children) != 1 {
				return fmt.Errorf("E_ARCHIVER")
			}
			tar := filepath.Join(output, children[0].Name())
			nested := filepath.Join(tmp, "expanded")
			os.Mkdir(nested, 0700)
			_, err = run(ctx, []string{"x", "-y", "-bsp1", "-o" + nested, "--", tar}, password, "", progress, tmp)
			output = nested
		}
		if err == nil {
			err = filepath.WalkDir(output, func(p string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				info, ie := d.Info()
				if ie != nil {
					return ie
				}
				if st, ok := info.Sys().(*syscall.Stat_t); ok && !d.IsDir() && st.Nlink > 1 {
					return fmt.Errorf("E_UNSAFE_ARCHIVE")
				}
				if d.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("E_UNSAFE_ARCHIVE")
				}
				return nil
			})
		}
	} else {
		if format != "zip" && format != "7z" && format != "tar.gz" {
			return fmt.Errorf("E_FORMAT")
		}
		if len(sources) == 0 {
			return fmt.Errorf("E_INPUT")
		}
		args := []string{"a", "-t" + format, "-sae", "-spd", "-bsp1", "-y"}
		if password != "" {
			if format == "tar.gz" {
				return fmt.Errorf("E_FORMAT")
			}
			args = append(args, "-p")
			if format == "7z" {
				args = append(args, "-mhe=on")
			} else {
				args = append(args, "-mem=AES256")
			}
		}
		base := filepath.Dir(sources[0])
		names := []string{}
		for _, p := range sources {
			if filepath.Dir(p) != base {
				return fmt.Errorf("E_INPUT")
			}
			rel, e := filepath.Rel(p, dest)
			if e == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
				return fmt.Errorf("E_OUTPUT")
			}
			if e := filepath.WalkDir(p, func(_ string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if d.Type()&os.ModeSymlink != 0 {
					return fmt.Errorf("E_UNSAFE_ARCHIVE")
				}
				return nil
			}); e != nil {
				return e
			}
			names = append(names, "./"+filepath.Base(p))
		}
		if format == "tar.gz" {
			args[1] = "-ttar"
		}
		args = append(args, "--", output)
		args = append(args, names...)
		_, err = run(ctx, args, password, base, progress, "")
		if err == nil && format == "tar.gz" {
			tar := filepath.Join(tmp, "payload.tar")
			err = os.Rename(output, tar)
			if err == nil {
				_, err = run(ctx, []string{"a", "-tgzip", "-sae", "-bsp1", "--", output, tar}, "", "", progress, "")
			}
		}
	}
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return fmt.Errorf("E_CANCELLED")
	}
	if err = publish(ctx, output, dest); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("E_CANCELLED")
		}
		if os.IsExist(err) {
			return fmt.Errorf("E_EXISTS")
		}
		return fmt.Errorf("E_OUTPUT")
	}
	return nil
}

// ExtractTo uses the same validated, sandboxed extraction as a user extraction job.
func ExtractTo(ctx context.Context, source, dest, password string) error {
	return (&Manager{}).work(ctx, []string{source}, dest, "", password, true)
}
