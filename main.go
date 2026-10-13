package main

import (
	"fmt"
	"github.com/egoist/mygo"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"zipgallery/internal/archive"
	"zipgallery/internal/archiver"
	"zipgallery/internal/gallery"
	"zipgallery/internal/location"
)

type Desktop struct {
	Core    *gallery.Service
	Jobs    archiver.Manager
	mu      sync.Mutex
	pending string
}

func (d *Desktop) OpenLocation(l location.Location) (gallery.View, error) {
	return d.Core.OpenLocation(l)
}
func (d *Desktop) Choose(title string) (gallery.View, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: title, Filters: []mygo.FileFilter{{Name: "Archives", Extensions: []string{"zip", "cbz", "7z", "rar", "tar", "gz", "bz2", "xz", "tgz"}}}})
	if err != nil || len(paths) == 0 {
		return gallery.View{}, err
	}
	return d.OpenLocation(location.Location{Path: paths[0]})
}
func (d *Desktop) Current() gallery.View     { return d.Core.Current() }
func (d *Desktop) Recents() []gallery.Recent { return d.Core.Recents() }
func (d *Desktop) Progress(session, id, folder string) error {
	return d.Core.Progress(session, id, folder)
}
func (d *Desktop) ClearCache() error { return d.Core.ClearCache() }
func (d *Desktop) Pending() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	p := d.pending
	d.pending = ""
	return p
}
func (d *Desktop) queue(p string) { d.mu.Lock(); d.pending = p; d.mu.Unlock() }
func (d *Desktop) PreviewRepair(path, codec string) (archive.RepairPreview, error) {
	return archive.PreviewRepair(path, codec)
}
func (d *Desktop) Repair(path, codec, version, title string) (string, error) {
	ext := filepath.Ext(path)
	dest, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Title: title, DefaultPath: strings.TrimSuffix(path, ext) + ".utf8" + ext})
	if err != nil || dest == "" {
		return "", err
	}
	if err = archive.RepairTo(path, dest, codec, version); err != nil {
		return "", err
	}
	return dest, nil
}
func (d *Desktop) OpenArchive(path, password string) (gallery.View, error) {
	return d.Core.OpenArchive(location.Location{Path: path}, password)
}
func (d *Desktop) SelectMounted(title string) (string, error) {
	p, e := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: title, DefaultPath: "/Volumes"})
	if e != nil || len(p) == 0 {
		return "", e
	}
	return p[0], nil
}
func (d *Desktop) SelectArchive(title string) (string, error) {
	p, e := mygo.Dialog.Open(mygo.OpenDialogOptions{Title: title})
	if e != nil || len(p) == 0 {
		return "", e
	}
	return p[0], nil
}
func (d *Desktop) SelectInputs(title string, directory bool) ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{Title: title, Directory: directory, Multiple: !directory})
}
func (d *Desktop) StartArchive(sources []string, format, password, title string, extract bool) (archiver.Job, error) {
	if len(sources) == 0 {
		return archiver.Job{}, fmt.Errorf("E_INPUT")
	}
	name := strings.TrimSuffix(filepath.Base(sources[0]), filepath.Ext(sources[0]))
	if !extract {
		name += "." + format
	} else {
		name += "-extracted"
	}
	dest, e := mygo.Dialog.Save(mygo.SaveDialogOptions{Title: title, DefaultPath: filepath.Join(filepath.Dir(sources[0]), name), CreateDirectories: true})
	if e != nil || dest == "" {
		return archiver.Job{}, e
	}
	if extract && password == "" {
		password = d.Core.ArchivePassword(sources[0])
	}
	return d.Jobs.Start(sources, dest, format, password, extract)
}
func (d *Desktop) JobStatus() archiver.Job { return d.Jobs.Status() }
func (d *Desktop) CancelJob()              { d.Jobs.Cancel() }
func main() {
	mygo.App.SetName("ZipGallery")
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	state, _ := os.UserConfigDir()
	cache, _ := os.UserCacheDir()
	if testDir := os.Getenv("ZIPGALLERY_DATA_DIR"); testDir != "" {
		state = testDir
		cache = testDir
	}
	d := &Desktop{Core: gallery.New(filepath.Join(state, "ZipGallery"), filepath.Join(cache, "ZipGallery", "thumbs"))}
	mygo.Bind(d)
	if err := mygo.Protocol.Handle("gallery", d.Core); err != nil {
		log.Fatal(err)
	}
	var win *mygo.Window
	mygo.App.OnOpenFile(func(p string) {
		d.queue(p)
		if win != nil {
			win.Show()
			win.Focus()
		}
	})
	mygo.App.OnSecondInstance(func(args []string, wd string) {
		if win != nil {
			win.Show()
			win.Focus()
		}
	})
	mygo.App.WhenReady(func() {
		win = mygo.NewWindow(mygo.WindowOptions{Title: "ZipGallery", Width: 1180, Height: 800, MinWidth: 760, MinHeight: 520, BackgroundColor: "#141719", StateKey: "main", URL: "/"})
	})
	mygo.App.OnWindowAllClosed(func() { mygo.App.Quit() })
	mygo.App.OnQuit(func() { d.Jobs.Cancel(); d.Core.Close() })
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
