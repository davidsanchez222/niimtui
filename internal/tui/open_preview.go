package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// previewViewer simulates a live-refreshing preview window for terminals without inline images.
// It rewrites one PNG in place and asks Preview.app to open it, so the window keeps its position
// and Preview reloads the changed file instead of a new window being created each time.
// Calls are serialized by the mutex and run in a tea.Cmd goroutine, never on the UI loop.
type previewViewer struct {
	mu   sync.Mutex
	dir  string
	seen string
}

var openPreviewViewer = &previewViewer{}

// openPreviewAvailable reports whether the native viewer can be driven. It is a variable so tests can stub it.
var openPreviewAvailable = func() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("open")
	return err == nil
}

func (v *previewViewer) show(png []byte, hash string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if hash != "" && hash == v.seen {
		return nil
	}
	if v.dir == "" {
		dir, err := os.MkdirTemp("", "niimtui-preview-")
		if err != nil {
			return err
		}
		v.dir = dir
	}
	path := filepath.Join(v.dir, "preview.png")
	// WriteFile truncates the existing file, keeping the inode Preview is watching.
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return err
	}
	v.seen = hash
	// Preview only repaints a changed file when it is activated, so `open -g` leaves the old pixels on
	// screen. Activate it, then hand focus straight back to whichever app (the terminal) had it.
	front := frontmostBundleID()
	if front == "" || front == previewBundleID {
		return exec.Command("open", "-a", "Preview", path).Run()
	}
	if err := exec.Command("open", "-a", "Preview", path).Run(); err != nil {
		return err
	}
	return exec.Command("open", "-b", front).Run()
}

const previewBundleID = "com.apple.Preview"

// frontmostBundleID returns the bundle identifier of the focused app via lsappinfo, which needs no
// Automation or Accessibility permission. It returns "" if it can't be determined.
func frontmostBundleID() string {
	asn, err := exec.Command("lsappinfo", "front").Output()
	if err != nil {
		return ""
	}
	out, err := exec.Command("lsappinfo", "info", "-only", "bundleid", strings.TrimSpace(string(asn))).Output()
	if err != nil {
		return ""
	}
	return parseBundleID(string(out))
}

func parseBundleID(out string) string {
	_, value, ok := strings.Cut(out, "=")
	if !ok {
		return ""
	}
	return strings.Trim(strings.TrimSpace(value), `"`)
}

func (v *previewViewer) close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.dir != "" {
		_ = os.RemoveAll(v.dir)
		v.dir = ""
	}
	v.seen = ""
}

func openPreviewRefreshCmd(png []byte, hash string) tea.Cmd {
	return func() tea.Msg {
		_ = openPreviewViewer.show(png, hash)
		return nil
	}
}
