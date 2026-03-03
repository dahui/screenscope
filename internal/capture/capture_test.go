package capture_test

import (
	"testing"

	"github.com/dahui/screenscope/internal/capture"
)

func TestConvertBGRAToRGBA_SingleRedPixel(t *testing.T) {
	// BGRA: B=0x00, G=0x00, R=0xFF, A=0x00 (alpha unused by X11)
	data := []byte{0x00, 0x00, 0xFF, 0x00}
	img, err := capture.ConvertBGRAToRGBA(data, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	r, g, b, a := img.At(0, 0).RGBA()
	assertColor(t, "red pixel", r, g, b, a, 0xFF, 0x00, 0x00, 0xFF)
}

func TestConvertBGRAToRGBA_SingleBluePixel(t *testing.T) {
	// BGRA: B=0xFF, G=0x00, R=0x00, A=0x00
	data := []byte{0xFF, 0x00, 0x00, 0x00}
	img, err := capture.ConvertBGRAToRGBA(data, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	r, g, b, a := img.At(0, 0).RGBA()
	assertColor(t, "blue pixel", r, g, b, a, 0x00, 0x00, 0xFF, 0xFF)
}

func TestConvertBGRAToRGBA_WhitePixel(t *testing.T) {
	data := []byte{0xFF, 0xFF, 0xFF, 0xFF}
	img, err := capture.ConvertBGRAToRGBA(data, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	r, g, b, a := img.At(0, 0).RGBA()
	assertColor(t, "white pixel", r, g, b, a, 0xFF, 0xFF, 0xFF, 0xFF)
}

func TestConvertBGRAToRGBA_BlackPixel(t *testing.T) {
	// BGRA: all zero. Alpha should still become 0xFF.
	data := []byte{0x00, 0x00, 0x00, 0x00}
	img, err := capture.ConvertBGRAToRGBA(data, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	r, g, b, a := img.At(0, 0).RGBA()
	assertColor(t, "black pixel", r, g, b, a, 0x00, 0x00, 0x00, 0xFF)
}

func TestConvertBGRAToRGBA_AlphaForcedOpaque(t *testing.T) {
	// Input alpha is 0x00 (transparent), output must be 0xFF (opaque).
	data := []byte{0x80, 0x40, 0x20, 0x00}
	img, err := capture.ConvertBGRAToRGBA(data, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	r, g, b, a := img.At(0, 0).RGBA()
	assertColor(t, "forced opaque", r, g, b, a, 0x20, 0x40, 0x80, 0xFF)
}

func TestConvertBGRAToRGBA_MultiPixel(t *testing.T) {
	// 2x2 image: red, green, blue, white
	data := []byte{
		0x00, 0x00, 0xFF, 0x00, // (0,0) red in BGRA
		0x00, 0xFF, 0x00, 0x00, // (1,0) green in BGRA
		0xFF, 0x00, 0x00, 0x00, // (0,1) blue in BGRA
		0xFF, 0xFF, 0xFF, 0xFF, // (1,1) white in BGRA
	}
	img, err := capture.ConvertBGRAToRGBA(data, 2, 2)
	if err != nil {
		t.Fatal(err)
	}

	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 2 {
		t.Fatalf("expected 2x2, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}

	r, g, b, a := img.At(0, 0).RGBA()
	assertColor(t, "(0,0) red", r, g, b, a, 0xFF, 0x00, 0x00, 0xFF)

	r, g, b, a = img.At(1, 0).RGBA()
	assertColor(t, "(1,0) green", r, g, b, a, 0x00, 0xFF, 0x00, 0xFF)

	r, g, b, a = img.At(0, 1).RGBA()
	assertColor(t, "(0,1) blue", r, g, b, a, 0x00, 0x00, 0xFF, 0xFF)

	r, g, b, a = img.At(1, 1).RGBA()
	assertColor(t, "(1,1) white", r, g, b, a, 0xFF, 0xFF, 0xFF, 0xFF)
}

func TestConvertBGRAToRGBA_ZeroDimension(t *testing.T) {
	img, err := capture.ConvertBGRAToRGBA([]byte{}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	if img.Bounds().Dx() != 0 || img.Bounds().Dy() != 0 {
		t.Errorf("expected 0x0, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestConvertBGRAToRGBA_BadLength(t *testing.T) {
	_, err := capture.ConvertBGRAToRGBA([]byte{0x00, 0x00}, 1, 1)
	if err == nil {
		t.Error("expected error for mismatched data length")
	}
}

func TestConvertBGRAToRGBA_TooMuchData(t *testing.T) {
	_, err := capture.ConvertBGRAToRGBA([]byte{0x00, 0x00, 0x00, 0x00, 0xFF}, 1, 1)
	if err == nil {
		t.Error("expected error for excess data")
	}
}

// assertColor checks that 16-bit pre-multiplied RGBA values match expected 8-bit values.
func assertColor(t *testing.T, label string, r, g, b, a uint32, wantR, wantG, wantB, wantA uint8) {
	t.Helper()
	gotR, gotG, gotB, gotA := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
	if gotR != wantR || gotG != wantG || gotB != wantB || gotA != wantA {
		t.Errorf("%s: got RGBA(%d,%d,%d,%d), want (%d,%d,%d,%d)",
			label, gotR, gotG, gotB, gotA, wantR, wantG, wantB, wantA)
	}
}
