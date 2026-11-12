package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// A cover chosen is kept square, from the middle of the picture, no
// larger than coverSide, and listed for its playlist.
func TestSaveCover(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1600, 1200))
	// Red at the sides the crop cuts off, blue in the middle.
	for y := range 1200 {
		for x := range 1600 {
			c := color.RGBA{0, 0, 255, 255}
			if x < 200 || x >= 1400 {
				c = color.RGBA{255, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	dir := t.TempDir()
	if _, err := saveCover(dir, 42, buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "42.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := got.Bounds(); b.Dx() != coverSide || b.Dy() != coverSide {
		t.Errorf("kept %v, want %d square", b, coverSide)
	}
	if r, _, bl, _ := got.At(5, coverSide/2).RGBA(); r > 0x4000 || bl < 0xc000 {
		t.Error("the crop kept the picture's sides")
	}
	if u := localCovers(dir)[42]; u == "" {
		t.Error("the cover kept is not listed")
	}
	if _, err := saveCover(dir, 43, []byte("not a picture")); err == nil {
		t.Error("kept something that is not a picture")
	}
}
