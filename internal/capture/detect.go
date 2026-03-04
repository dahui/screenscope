package capture

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/jezek/xgb"
)

const x11SocketDir = "/tmp/.X11-unix"

// detectDisplay scans /tmp/.X11-unix/ for X11 server sockets and returns
// the first display string (e.g. ":0") that accepts a connection.
func detectDisplay() (string, error) {
	entries, err := os.ReadDir(x11SocketDir)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", x11SocketDir, err)
	}

	displays := parseDisplays(entries)
	if len(displays) == 0 {
		return "", fmt.Errorf("no X11 sockets found in %s", x11SocketDir)
	}

	for _, d := range displays {
		conn, connErr := xgb.NewConnDisplay(d)
		if connErr == nil {
			conn.Close()
			return d, nil
		}
	}

	return "", fmt.Errorf("found %d X11 socket(s) in %s but none accepted a connection", len(displays), x11SocketDir)
}

// parseDisplays extracts display strings from X11 socket directory entries.
// Entries named "X<number>" are converted to ":<number>" and returned sorted
// numerically in ascending order.
func parseDisplays(entries []os.DirEntry) []string {
	var displays []string

	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "X") {
			continue
		}
		numStr := name[1:]
		if _, err := strconv.Atoi(numStr); err != nil {
			continue
		}
		displays = append(displays, ":"+numStr)
	}

	sort.Slice(displays, func(i, j int) bool {
		ni, _ := strconv.Atoi(displays[i][1:])
		nj, _ := strconv.Atoi(displays[j][1:])
		return ni < nj
	})

	return displays
}
