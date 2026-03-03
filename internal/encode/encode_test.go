package encode_test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/dahui/screenscope/internal/encode"
)

func TestWritePNG_RoundTrip(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	path := filepath.Join(t.TempDir(), "test.png")
	if err := encode.WritePNG(img, path); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}()

	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}

	if decoded.Bounds() != img.Bounds() {
		t.Errorf("bounds mismatch: got %v, want %v", decoded.Bounds(), img.Bounds())
	}

	// Verify red pixel at (0,0).
	r, g, b, a := decoded.At(0, 0).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("pixel (0,0): got (%d,%d,%d,%d), want (255,0,0,255)", r>>8, g>>8, b>>8, a>>8)
	}

	// Verify white pixel at (1,1).
	r, g, b, a = decoded.At(1, 1).RGBA()
	if r>>8 != 255 || g>>8 != 255 || b>>8 != 255 || a>>8 != 255 {
		t.Errorf("pixel (1,1): got (%d,%d,%d,%d), want (255,255,255,255)", r>>8, g>>8, b>>8, a>>8)
	}
}

func TestWritePNG_InvalidPath(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	err := encode.WritePNG(img, "/nonexistent/dir/test.png")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestWritePNG_CreatesValidFile(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	path := filepath.Join(t.TempDir(), "output.png")

	if err := encode.WritePNG(img, path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() == 0 {
		t.Error("output file is empty")
	}
}
