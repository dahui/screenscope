package capture

import (
	"io/fs"
	"testing"
)

// fakeDirEntry implements os.DirEntry for testing parseDisplays.
type fakeDirEntry struct {
	name string
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                 { return false }
func (f fakeDirEntry) Type() fs.FileMode           { return 0 }
func (f fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

func TestParseDisplays(t *testing.T) {
	entries := []fakeDirEntry{
		{name: "X0"},
		{name: "X1"},
		{name: "X42"},
	}

	dirEntries := make([]fs.DirEntry, len(entries))
	for i := range entries {
		dirEntries[i] = entries[i]
	}

	displays := parseDisplays(dirEntries)

	want := []string{":0", ":1", ":42"}
	if len(displays) != len(want) {
		t.Fatalf("got %d displays, want %d", len(displays), len(want))
	}
	for i, d := range displays {
		if d != want[i] {
			t.Errorf("displays[%d] = %q, want %q", i, d, want[i])
		}
	}
}

func TestParseDisplays_SkipsNonDisplayFiles(t *testing.T) {
	entries := []fakeDirEntry{
		{name: ".X0-lock"},
		{name: "Xfoo"},
		{name: "something"},
		{name: "X"},
		{name: "X1"},
	}

	dirEntries := make([]fs.DirEntry, len(entries))
	for i := range entries {
		dirEntries[i] = entries[i]
	}

	displays := parseDisplays(dirEntries)

	if len(displays) != 1 || displays[0] != ":1" {
		t.Errorf("got %v, want [\":1\"]", displays)
	}
}

func TestParseDisplays_Empty(t *testing.T) {
	displays := parseDisplays(nil)
	if len(displays) != 0 {
		t.Errorf("got %v, want empty", displays)
	}
}

func TestParseDisplays_SortsNumerically(t *testing.T) {
	entries := []fakeDirEntry{
		{name: "X10"},
		{name: "X2"},
		{name: "X1"},
	}

	dirEntries := make([]fs.DirEntry, len(entries))
	for i := range entries {
		dirEntries[i] = entries[i]
	}

	displays := parseDisplays(dirEntries)

	want := []string{":1", ":2", ":10"}
	if len(displays) != len(want) {
		t.Fatalf("got %d displays, want %d", len(displays), len(want))
	}
	for i, d := range displays {
		if d != want[i] {
			t.Errorf("displays[%d] = %q, want %q", i, d, want[i])
		}
	}
}
