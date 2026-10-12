// Measures backend index, thumbnail batch and image latency. Not UI paint time.
package main

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
	"zipgallery/internal/gallery"
	"zipgallery/internal/location"
)

func main() {
	if len(os.Args) < 2 {
		panic("usage: benchmark archive")
	}
	dir, _ := os.MkdirTemp("", "zipgallery-bench-")
	defer os.RemoveAll(dir)
	s := gallery.New(dir, filepath.Join(dir, "thumbs"))
	defer s.Close()
	start := time.Now()
	v, err := s.OpenLocation(location.Location{Path: os.Args[1]})
	if err != nil {
		panic(err)
	}
	fmt.Printf("index: %s; entries: %d\n", time.Since(start), len(v.Entries))
	request := func(kind string, i int) {
		r := httptest.NewRecorder()
		s.ServeHTTP(r, httptest.NewRequest("GET", "gallery://localhost/"+kind+"/"+v.Entries[i].ID, nil))
		if r.Code != 200 {
			panic(r.Body.String())
		}
	}
	batch := func() {
		var wg sync.WaitGroup
		for i := 0; i < min(12, len(v.Entries)); i++ {
			wg.Add(1)
			go func(i int) { defer wg.Done(); request("thumb", i) }(i)
		}
		wg.Wait()
	}
	start = time.Now()
	batch()
	fmt.Println("12 thumbnails cold:", time.Since(start))
	start = time.Now()
	batch()
	fmt.Println("12 thumbnails memory warm:", time.Since(start))
	var latency []time.Duration
	for i := 0; i < 20; i++ {
		start = time.Now()
		request("full", i%len(v.Entries))
		latency = append(latency, time.Since(start))
	}
	sort.Slice(latency, func(i, j int) bool { return latency[i] < latency[j] })
	fmt.Printf("full decode 20 pages median: %s; max: %s\n", latency[10], latency[19])
	var size int64
	filepath.Walk(filepath.Join(dir, "thumbs"), func(_ string, f os.FileInfo, err error) error {
		if err == nil && !f.IsDir() {
			size += f.Size()
		}
		return nil
	})
	fmt.Println("thumbnail disk bytes:", size)
}
