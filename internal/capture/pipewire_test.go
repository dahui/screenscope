package capture

import (
	"testing"
)

func TestConvertRGBxToRGBA_SinglePixel(t *testing.T) {
	// RGBx: R=0xFF, G=0x00, B=0x80, x=0x00 (padding)
	data := []byte{0xFF, 0x00, 0x80, 0x00}
	img, err := convertRGBxToRGBA(data, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	r, g, b, a := img.At(0, 0).RGBA()
	if uint8(r>>8) != 0xFF || uint8(g>>8) != 0x00 || uint8(b>>8) != 0x80 || uint8(a>>8) != 0xFF {
		t.Errorf("got RGBA(%d,%d,%d,%d), want (255,0,128,255)",
			uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
	}
}

func TestConvertRGBxToRGBA_AlphaForcedOpaque(t *testing.T) {
	// The 'x' byte (index 3) should always become 0xFF regardless of input.
	data := []byte{0x40, 0x80, 0xC0, 0x00}
	img, err := convertRGBxToRGBA(data, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, a := img.At(0, 0).RGBA()
	if uint8(a>>8) != 0xFF {
		t.Errorf("alpha = %d, want 255", uint8(a>>8))
	}
}

func TestConvertRGBxToRGBA_MultiPixel(t *testing.T) {
	data := []byte{
		0xFF, 0x00, 0x00, 0x00, // red
		0x00, 0xFF, 0x00, 0x00, // green
		0x00, 0x00, 0xFF, 0x00, // blue
		0xFF, 0xFF, 0xFF, 0x00, // white
	}
	img, err := convertRGBxToRGBA(data, 2, 2)
	if err != nil {
		t.Fatal(err)
	}

	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 2 {
		t.Fatalf("expected 2x2, got %dx%d", img.Bounds().Dx(), img.Bounds().Dy())
	}

	r, g, b, _ := img.At(0, 0).RGBA()
	if uint8(r>>8) != 0xFF || uint8(g>>8) != 0x00 || uint8(b>>8) != 0x00 {
		t.Errorf("(0,0): got RGB(%d,%d,%d), want (255,0,0)", uint8(r>>8), uint8(g>>8), uint8(b>>8))
	}
}

func TestConvertRGBxToRGBA_BadLength(t *testing.T) {
	_, err := convertRGBxToRGBA([]byte{0x00, 0x00}, 1, 1)
	if err == nil {
		t.Error("expected error for mismatched data length")
	}
}

func TestConvertRGBADirect_SinglePixel(t *testing.T) {
	data := []byte{0xFF, 0x00, 0x80, 0xCC}
	img, err := convertRGBADirect(data, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	r, g, b, a := img.At(0, 0).RGBA()
	if uint8(r>>8) != 0xFF || uint8(g>>8) != 0x00 || uint8(b>>8) != 0x80 || uint8(a>>8) != 0xCC {
		t.Errorf("got RGBA(%d,%d,%d,%d), want (255,0,128,204)",
			uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
	}
}

func TestConvertRGBADirect_PreservesAlpha(t *testing.T) {
	// Unlike RGBx, RGBA should preserve the alpha channel as-is.
	data := []byte{0x00, 0x00, 0x00, 0x80}
	img, err := convertRGBADirect(data, 1, 1)
	if err != nil {
		t.Fatal(err)
	}

	_, _, _, a := img.At(0, 0).RGBA()
	if uint8(a>>8) != 0x80 {
		t.Errorf("alpha = %d, want 128", uint8(a>>8))
	}
}

func TestConvertRGBADirect_BadLength(t *testing.T) {
	_, err := convertRGBADirect([]byte{0x00, 0x00}, 1, 1)
	if err == nil {
		t.Error("expected error for mismatched data length")
	}
}
