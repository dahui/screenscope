// Package capture provides screen capture via PipeWire (gamescope-session)
// with an X11 fallback for traditional desktops.
package capture

import (
	"fmt"
	"image"
	"os"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// Error is a capture error that carries a user-facing hint alongside the
// underlying error. The hint provides actionable context that helps the
// user understand what went wrong and how to fix it.
type Error struct {
	Err  error
	Hint string
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Screen captures the entire screen. It tries PipeWire first (which works
// inside gamescope-session), then falls back to X11 GetImage (which works on
// traditional X11 desktops).
//
// If display is empty, the $DISPLAY environment variable is used for the X11
// fallback. If $DISPLAY is also empty, the function scans /tmp/.X11-unix/ for
// available X11 servers and connects to the first one that responds.
func Screen(display string) (*image.RGBA, error) {
	// Try PipeWire first (works in gamescope-session).
	img, pwErr := ViaPipeWire()
	if pwErr == nil {
		return img, nil
	}

	// Fall back to X11 GetImage (works on traditional X11 desktops).
	img, x11Err := captureX11(display)
	if x11Err == nil {
		return img, nil
	}

	// Both methods failed. Return a combined error.
	return nil, &Error{
		Err: fmt.Errorf("screen capture failed: pipewire: %v; x11: %v", pwErr, x11Err),
		Hint: fmt.Sprintf("PipeWire capture failed: %v\n", pwErr) +
			fmt.Sprintf("X11 capture also failed: %v\n\n", x11Err) +
			"If running inside gamescope-session, ensure PipeWire is running and\n" +
			"gamescope is exposing a Video/Source node.\n\n" +
			"If running on a traditional X11 desktop, ensure $DISPLAY is set.",
	}
}

// captureX11 captures the screen via X11 GetImage on the root window.
func captureX11(display string) (*image.RGBA, error) {
	if display == "" {
		display = os.Getenv("DISPLAY")
	}
	if display == "" {
		detected, err := detectDisplay()
		if err != nil {
			return nil, fmt.Errorf("could not find an X11 display: %w", err)
		}
		display = detected
	}

	conn, err := xgb.NewConnDisplay(display)
	if err != nil {
		return nil, fmt.Errorf("could not connect to X11 display %s: %w", display, err)
	}
	defer conn.Close()

	screen := xproto.Setup(conn).DefaultScreen(conn)
	width := screen.WidthInPixels
	height := screen.HeightInPixels

	if width == 0 || height == 0 {
		return nil, fmt.Errorf("X11 screen reported zero dimensions (%dx%d)", width, height)
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
		return nil, fmt.Errorf("could not capture screen: %w", err)
	}

	img, err := ConvertBGRAToRGBA(reply.Data, int(width), int(height))
	if err != nil {
		return nil, fmt.Errorf("unexpected pixel format from X11 server: %w", err)
	}

	return img, nil
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

