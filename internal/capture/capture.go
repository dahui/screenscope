// Package capture provides X11 screen capture using the xgb library.
package capture

import (
	"fmt"
	"image"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// Screen captures the entire screen from the X11 display.
// If display is empty, the $DISPLAY environment variable is used.
func Screen(display string) (*image.RGBA, error) {
	conn, err := xgb.NewConnDisplay(display)
	if err != nil {
		return nil, fmt.Errorf("connect to X11 display: %w (is $DISPLAY set?)", err)
	}
	defer conn.Close()

	screen := xproto.Setup(conn).DefaultScreen(conn)
	width := screen.WidthInPixels
	height := screen.HeightInPixels

	if width == 0 || height == 0 {
		return nil, fmt.Errorf("screen has zero dimensions: %dx%d", width, height)
	}

	reply, err := xproto.GetImage(
		conn,
		xproto.ImageFormatZPixmap,
		xproto.Drawable(screen.Root),
		0, 0,
		width, height,
		0xFFFFFFFF,
	).Reply()
	if err != nil {
		return nil, fmt.Errorf("get image: %w", err)
	}

	return ConvertBGRAToRGBA(reply.Data, int(width), int(height))
}

// ConvertBGRAToRGBA converts raw BGRA pixel data (as returned by X11 ZPixmap
// on 32-bit TrueColor visuals) to a Go image.RGBA. Alpha is forced to 0xFF
// because X11 root windows have an unused alpha channel.
func ConvertBGRAToRGBA(data []byte, width, height int) (*image.RGBA, error) {
	expected := width * height * 4
	if len(data) != expected {
		return nil, fmt.Errorf("pixel data length %d does not match %dx%dx4=%d", len(data), width, height, expected)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(data); i += 4 {
		img.Pix[i+0] = data[i+2] // R <- B position
		img.Pix[i+1] = data[i+1] // G stays
		img.Pix[i+2] = data[i+0] // B <- R position
		img.Pix[i+3] = 0xFF      // A = fully opaque
	}

	return img, nil
}
