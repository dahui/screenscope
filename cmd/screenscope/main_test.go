package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveOutput_ExplicitFile(t *testing.T) {
	path, err := resolveOutput("/tmp/screenshot.png", "")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/screenshot.png" {
		t.Errorf("got %q, want /tmp/screenshot.png", path)
	}
}

func TestResolveOutput_Directory(t *testing.T) {
	dir := t.TempDir()
	path, err := resolveOutput("", dir)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(path, dir) {
		t.Errorf("path %q does not start with dir %q", path, dir)
	}
	if !strings.HasPrefix(filepath.Base(path), "screenscope_") {
		t.Errorf("filename %q does not start with screenscope_", filepath.Base(path))
	}
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("path %q does not end with .png", path)
	}
}

func TestResolveOutput_DirectoryNotExist(t *testing.T) {
	_, err := resolveOutput("", "/nonexistent/dir/that/should/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestResolveOutput_NotADirectory(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "not-a-dir")
	if err != nil {
		t.Fatal(err)
	}
	if closeErr := f.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}

	_, err = resolveOutput("", f.Name())
	if err == nil {
		t.Error("expected error when dir is actually a file")
	}
}

func TestResolveOutput_DefaultCurrentDir(t *testing.T) {
	path, err := resolveOutput("", "")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(path, "screenscope_") {
		t.Errorf("path %q does not start with screenscope_", path)
	}
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("path %q does not end with .png", path)
	}
	// Should be a bare filename (no directory component).
	if filepath.Dir(path) != "." {
		t.Errorf("expected current dir (.), got %q", filepath.Dir(path))
	}
}
