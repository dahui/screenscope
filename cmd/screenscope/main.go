// Screenscope captures screenshots from X11 displays, including inside
// gamescope-session where standard screenshot tools fail.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/dahui/screenscope/internal/capture"
	"github.com/dahui/screenscope/internal/encode"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	var (
		output  string
		dir     string
		delay   int
		version bool
		help    bool
	)

	flag.StringVarP(&output, "file", "f", "", "write screenshot to this file path")
	flag.StringVarP(&dir, "dir", "d", "", "write screenshot to this directory with an auto-generated filename")
	flag.IntVarP(&delay, "delay", "D", 0, "wait this many seconds before capturing")
	flag.BoolVar(&version, "version", false, "print version and exit")
	flag.BoolVarP(&help, "help", "h", false, "show this help message")
	flag.Usage = usage
	flag.Parse()

	if help {
		usage()
		return
	}

	if version {
		fmt.Printf("screenscope %s\n", Version)
		return
	}

	if output != "" && dir != "" {
		fmt.Fprintln(os.Stderr, "error: --file and --dir are mutually exclusive")
		flag.Usage()
		os.Exit(1)
	}

	path, err := resolveOutput(output, dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Second)
	}

	img, err := capture.Screen("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := encode.WritePNG(img, path); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(path)
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: screenscope [flags]

Capture a screenshot from the current X11 display. Designed for use inside
gamescope-session where standard screenshot tools fail.

If neither --file nor --dir is specified, the screenshot is saved to the
current directory with an auto-generated filename (screenscope_DATE_TIME.png).

The --file and --dir flags are mutually exclusive.

Flags:
`)
	flag.PrintDefaults()
}

// resolveOutput determines the output file path from the provided flags.
func resolveOutput(output, dir string) (string, error) {
	if output != "" {
		return output, nil
	}

	name := time.Now().Format("screenscope_20060102_150405.png")

	if dir != "" {
		info, err := os.Stat(dir)
		if err != nil {
			return "", fmt.Errorf("output directory: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%s is not a directory", dir)
		}
		return filepath.Join(dir, name), nil
	}

	return name, nil
}
