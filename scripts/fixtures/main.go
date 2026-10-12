// Generate only synthetic, non-sensitive fixtures. Large archive is opt-in.
package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/encoding/simplifiedchinese"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
)

func picture(n int) image.Image {
	im := image.NewRGBA(image.Rect(0, 0, 1000, 700))
	palette := []color.RGBA{{68, 113, 96, 255}, {165, 106, 74, 255}, {68, 95, 143, 255}}
	c := palette[n%3]
	for y := 0; y < 700; y++ {
		for x := 0; x < 1000; x++ {
			col := c
			if (x/70+y/70)%2 == 0 {
				col.R += 20
				col.G += 20
				col.B += 20
			}
			im.SetRGBA(x, y, col)
		}
	}
	d := font.Drawer{Dst: im, Src: image.White, Face: basicfont.Face7x13, Dot: fixed.P(50, 60)}
	d.DrawString(fmt.Sprintf("ZIPGALLERY / SYNTHETIC IMAGE %02d", n))
	return im
}
func main() {
	large := flag.Bool("large", false, "also create ~3GB / 1200 image archive")
	flag.Parse()
	os.MkdirAll("fixtures", 0755)
	var jpg, pngb bytes.Buffer
	jpeg.Encode(&jpg, picture(1), &jpeg.Options{Quality: 90})
	png.Encode(&pngb, picture(2))
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	names := []string{"1.jpg", "2.png", "10.jpg", "旅の記録/1.jpg", "旅の記録/2.png", "旅の記録/10.jpg", "旅の記録/海辺/1.jpg", "另一个目录/1.jpg", "broken.jpg", "__MACOSX/ignored.jpg", "._ignored.jpg"}
	for _, n := range names {
		w, _ := z.Create(n)
		if n == "broken.jpg" {
			w.Write([]byte("not an image"))
		} else if filepath.Ext(n) == ".png" {
			w.Write(pngb.Bytes())
		} else {
			w.Write(jpg.Bytes())
		}
	}
	z.Close()
	for _, ext := range []string{"zip", "cbz"} {
		os.WriteFile("fixtures/sample."+ext, b.Bytes(), 0644)
	}
	os.WriteFile("fixtures/corrupt.zip", []byte("not zip"), 0644)
	var empty bytes.Buffer
	ez := zip.NewWriter(&empty)
	ew, _ := ez.Create("readme.txt")
	ew.Write([]byte("no images"))
	ez.Close()
	os.WriteFile("fixtures/no-images.zip", empty.Bytes(), 0644)
	if *large {
		im := image.NewRGBA(image.Rect(0, 0, 1800, 1200))
		for i := 0; i < len(im.Pix); i += 4 {
			im.Pix[i] = byte(rand.Uint32())
			im.Pix[i+1] = byte(rand.Uint32())
			im.Pix[i+2] = byte(rand.Uint32())
			im.Pix[i+3] = 255
		}
		var noise bytes.Buffer
		jpeg.Encode(&noise, im, &jpeg.Options{Quality: 95})
		f, _ := os.Create("fixtures/large.cbz")
		w := zip.NewWriter(f)
		for i := 0; i < 1200; i++ {
			a, _ := w.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("%04d.jpg", i+1), Method: zip.Store})
			a.Write(noise.Bytes())
		}
		w.Close()
		f.Close()
		fmt.Printf("large: 1200 images, 1800×1200, JPEG Q95, Store, %d bytes/image\n", noise.Len())
	}
	var legacy bytes.Buffer
	lz := zip.NewWriter(&legacy)
	rawName, _ := simplifiedchinese.GB18030.NewEncoder().String("测试图片/第一页.jpg")
	lw, _ := lz.CreateHeader(&zip.FileHeader{Name: rawName, NonUTF8: true, Method: zip.Deflate})
	lw.Write(jpg.Bytes())
	lz.Close()
	os.WriteFile("fixtures/legacy-gbk.zip", legacy.Bytes(), 0644)
	fmt.Println("Fixtures ready in fixtures/")
}
