// Package images owns bounded decoding, priority scheduling and encoded caches.
package images

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
	"zipgallery/internal/archive"
)

const MaxPixels = 24_000_000
const memoryLimit = 32 << 20
const diskLimit = 256 << 20

type cached struct {
	data []byte
	used time.Time
}
type job struct {
	key        string
	pendingKey string
	s          *archive.Session
	e          archive.Entry
	thumb      bool
	priority   int
	callers    []context.Context
	done       chan struct{}
	data       []byte
	err        error
}
type Engine struct {
	mu         sync.Mutex
	cond       *sync.Cond
	queue      []*job
	pending    map[string]*job
	cache      map[string]cached
	bytes      int
	dir        string
	diskMu     sync.Mutex
	generation uint64
}

func New(dir string) *Engine {
	e := &Engine{dir: dir, pending: map[string]*job{}, cache: map[string]cached{}}
	e.cond = sync.NewCond(&e.mu)
	_ = os.MkdirAll(dir, 0700)
	for range 2 {
		go e.worker()
	}
	return e
}
func (e *Engine) Get(ctx context.Context, s *archive.Session, a archive.Entry, thumb bool, priority int) ([]byte, error) {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("v1:%s:%s:%s:%v", s.Source.Location.Path, s.Source.Version, a.Identity, thumb))))
	e.mu.Lock()
	if c, ok := e.cache[key]; ok {
		c.used = time.Now()
		e.cache[key] = c
		e.mu.Unlock()
		return c.data, nil
	}
	pendingKey := s.ID + ":" + key
	j := e.pending[pendingKey]
	if j == nil {
		j = &job{key: key, pendingKey: pendingKey, s: s, e: a, thumb: thumb, priority: priority, done: make(chan struct{})}
		e.pending[pendingKey] = j
		e.queue = append(e.queue, j)
	} else if priority < j.priority {
		j.priority = priority
	}
	j.callers = append(j.callers, ctx)
	e.cond.Signal()
	e.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.Context.Done():
		return nil, s.Context.Err()
	case <-j.done:
		return j.data, j.err
	}
}
func (e *Engine) worker() {
	for {
		e.mu.Lock()
		for len(e.queue) == 0 {
			e.cond.Wait()
		}
		best := 0
		for i, j := range e.queue {
			if j.priority < e.queue[best].priority {
				best = i
			}
		}
		j := e.queue[best]
		e.queue = append(e.queue[:best], e.queue[best+1:]...)
		live := false
		for _, c := range j.callers {
			if c.Err() == nil {
				live = true
			}
		}
		gen := e.generation
		e.mu.Unlock()
		if !live || j.s.Context.Err() != nil {
			j.err = context.Canceled
		} else {
			j.data, j.err = e.render(j, gen)
		}
		e.mu.Lock()
		if j.err == nil && gen == e.generation {
			e.bytes -= len(e.cache[j.key].data)
			e.cache[j.key] = cached{j.data, time.Now()}
			e.bytes += len(j.data)
			for e.bytes > memoryLimit {
				var oldest string
				var t time.Time
				for k, c := range e.cache {
					if oldest == "" || c.used.Before(t) {
						oldest = k
						t = c.used
					}
				}
				e.bytes -= len(e.cache[oldest].data)
				delete(e.cache, oldest)
			}
		}
		delete(e.pending, j.pendingKey)
		close(j.done)
		e.mu.Unlock()
	}
}
func (e *Engine) render(j *job, gen uint64) ([]byte, error) {
	file := filepath.Join(e.dir, j.key+".jpg")
	if j.thumb && !j.s.Private {
		e.diskMu.Lock()
		b, err := os.ReadFile(file)
		if err == nil {
			_ = os.Chtimes(file, time.Now(), time.Now())
		}
		e.diskMu.Unlock()
		if err == nil {
			return b, nil
		}
	}
	b, err := j.s.Read(j.e)
	if err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("E_IMAGE: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, fmt.Errorf("E_PIXELS")
	}
	if err = j.s.Context.Err(); err != nil {
		return nil, err
	}
	img, err := imaging.Decode(bytes.NewReader(b), imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("E_IMAGE: %w", err)
	}
	if j.thumb {
		img = imaging.Fit(img, 320, 240, imaging.Linear)
	}
	var out bytes.Buffer
	if err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	if err = j.s.Context.Err(); err != nil {
		return nil, err
	}
	result := out.Bytes()
	if j.thumb && !j.s.Private {
		e.diskMu.Lock()
		e.mu.Lock()
		valid := gen == e.generation
		e.mu.Unlock()
		if valid {
			if os.WriteFile(file, result, 0600) == nil {
				e.trimDisk()
			}
		}
		e.diskMu.Unlock()
	}
	return result, nil
}
func (e *Engine) trimDisk() {
	files, _ := os.ReadDir(e.dir)
	type f struct {
		n string
		s int64
		t time.Time
	}
	var fs []f
	var total int64
	for _, x := range files {
		if filepath.Ext(x.Name()) != ".jpg" {
			continue
		}
		st, err := x.Info()
		if err == nil {
			total += st.Size()
			fs = append(fs, f{x.Name(), st.Size(), st.ModTime()})
		}
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].t.Before(fs[j].t) })
	for _, x := range fs {
		if total <= diskLimit {
			break
		}
		if os.Remove(filepath.Join(e.dir, x.n)) == nil {
			total -= x.s
		}
	}
}
func (e *Engine) Clear() error {
	e.diskMu.Lock()
	defer e.diskMu.Unlock()
	e.mu.Lock()
	e.generation++
	e.cache = map[string]cached{}
	e.bytes = 0
	e.mu.Unlock()
	files, err := os.ReadDir(e.dir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".jpg" {
			if err := os.Remove(filepath.Join(e.dir, f.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
