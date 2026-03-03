// Package capture provides X11 screen capture using the xgb library.
package capture

import (
	"errors"
	"fmt"
	"image"

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

// Screen captures the entire screen from the X11 display.
// If display is empty, the $DISPLAY environment variable is used.
func Screen(display string) (*image.RGBA, error) {
	conn, err := xgb.NewConnDisplay(display)
	if err != nil {
		return nil, &Error{
			Err:  fmt.Errorf("could not connect to X11 display: %w", err),
			Hint: "Is $DISPLAY set? This tool requires an X11 display (e.g. gamescope-session).",
		}
	}
	defer conn.Close()

	screen := xproto.Setup(conn).DefaultScreen(conn)
	width := screen.WidthInPixels
	height := screen.HeightInPixels

	if width == 0 || height == 0 {
		return nil, &Error{
			Err:  fmt.Errorf("X11 screen reported zero dimensions (%dx%d)", width, height),
			Hint: "The X11 display may not be fully initialized.",
		}
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
		return nil, &Error{
			Err:  fmt.Errorf("could not capture screen: %w", err),
			Hint: getImageHint(err),
		}
	}

	img, err := ConvertBGRAToRGBA(reply.Data, int(width), int(height))
	if err != nil {
		return nil, &Error{
			Err:  fmt.Errorf("unexpected pixel format from X11 server: %w", err),
			Hint: "The display is using an unsupported color depth.",
		}
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

func getImageHint(err error) string {
	var matchErr xproto.MatchError
	if errors.As(err, &matchErr) {
		return "The X11 server refused to capture the root window. This typically\n" +
			"happens on Wayland desktops where Xwayland does not expose the\n" +
			"composited screen through X11. screenscope is designed for use\n" +
			"inside gamescope-session."
	}

	var drawableErr xproto.DrawableError
	if errors.As(err, &drawableErr) {
		return "The X11 root window is not a valid drawable. The display server\n" +
			"may not support direct screen capture."
	}

	return "An unexpected X11 error occurred while capturing the screen."
}
