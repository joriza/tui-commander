//go:build windows

package core

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Long paths beyond MAX_PATH (260 characters) on Windows.
//
// This behaviour is easy to get wrong in two opposite ways, and the repository
// shipped with one of them: core.ExtendLongPath exists, is covered by tests,
// and is called from nowhere.
//
// What actually happens, checked against the Go 1.26 sources and by running the
// calls: the runtime sets the IsLongPathAwareProcess bit in the process's PEB
// at startup and then flips internal/syscall/windows.CanUseLongPaths to true
// (runtime/os_windows.go, initLongPathSupport). That makes os.fixLongPath a
// no-op -- the os package deliberately does NOT add the `\\?\` prefix -- and
// Windows extends the path for the process instead. So every os.* call in this
// package already handles a long path, and so does a raw Win32 call such as
// SHFileOperationW, because the process is long-path aware.
//
// The consequence for this repository: ExtendLongPath must NOT be wired into
// the call sites. It would duplicate what the runtime already arranges, and
// applying it would leak a `\\?\` prefix into returned *os.PathError values,
// into dialog text and into the path handed to the external editor, for no gain.
//
// The remaining dependency is environmental rather than ours: Windows honours
// the long-path-aware flag together with the machine-wide LongPathsEnabled
// registry value (HKLM\SYSTEM\CurrentControlSet\Control\FileSystem). These tests
// assert the behaviour on a machine where long paths are enabled, which is what
// the shipped product needs.

// deepTree creates a directory tree whose path exceeds MAX_PATH and returns the
// directory and the path of a text file inside it. The tree is built with
// ordinary os calls on purpose: if Go needed a hand-rolled prefix, the setup
// itself would be the first thing to fail.
func deepTree(t *testing.T) (dir, file string) {
	t.Helper()

	dir = t.TempDir()
	for i := 0; i < 12; i++ {
		dir = filepath.Join(dir, strings.Repeat("segment", 3)+strconv.Itoa(i))
	}
	if len(dir) <= 260 {
		t.Fatalf("test setup: path is only %d characters, more than 260 are needed", len(dir))
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create a %d-character directory tree: %v", len(dir), err)
	}

	file = filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(file, []byte("deep contents"), 0o644); err != nil {
		t.Fatalf("create a file at a %d-character path: %v", len(file), err)
	}
	return dir, file
}

// TestLongPath_Listing loads a directory beyond MAX_PATH through Panel.Load and
// checks its entries, including the metadata that comes from DirEntry.Info(),
// which resolves one more long path per entry.
func TestLongPath_Listing(t *testing.T) {
	dir, file := deepTree(t)

	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatalf("create subdirectory: %v", err)
	}

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Panel.Load on a %d-character path: %v", len(dir), err)
	}

	byName := make(map[string]Entry)
	for _, e := range p.Entries() {
		byName[e.Name] = e
	}

	sub, ok := byName["sub"]
	if !ok {
		t.Fatalf("subdirectory missing from the listing of a %d-character path", len(dir))
	}
	if !sub.IsDir {
		t.Error("subdirectory entry is not marked as a directory")
	}
	if sub.ModTime.IsZero() {
		t.Error("subdirectory entry has no modification time: DirEntry.Info() failed on the long path")
	}

	f, ok := byName[filepath.Base(file)]
	if !ok {
		t.Fatalf("file missing from the listing of a %d-character path", len(dir))
	}
	if f.IsDir {
		t.Error("file entry is marked as a directory")
	}
	if want := int64(len("deep contents")); f.Size != want {
		t.Errorf("file size = %d, want %d", f.Size, want)
	}
}

// TestLongPath_Preview reads a file beyond MAX_PATH through ReadTextPreview.
func TestLongPath_Preview(t *testing.T) {
	_, file := deepTree(t)

	preview, err := ReadTextPreview(file, 1<<20)
	if err != nil {
		t.Fatalf("ReadTextPreview on a %d-character path: %v", len(file), err)
	}
	if preview.Binary {
		t.Error("plain text was detected as binary")
	}
	if preview.Text != "deep contents" {
		t.Errorf("preview text = %q, want %q", preview.Text, "deep contents")
	}
}

// TestLongPath_Trash sends a file beyond MAX_PATH through the real default
// delete policy. A fixed volume routes to the recycle bin, which ends in a raw
// SHFileOperationW call -- the one filesystem call in this program that does not
// go through the os package, and therefore the only place where the runtime's
// long-path support cannot be taken for granted.
//
// It is opt-in: it writes to the real Recycle Bin, and a test suite must not
// litter the machine it runs on.
func TestLongPath_Trash(t *testing.T) {
	if os.Getenv("TUI_COMMANDER_TEST_TRASH") == "" {
		t.Skip("set TUI_COMMANDER_TEST_TRASH=1 to exercise the real Recycle Bin")
	}

	_, file := deepTree(t)

	if mode := DefaultDeleteMode(file); mode != DeleteToTrash {
		t.Skipf("the temporary volume is not fixed, so the policy deletes permanently (mode %v)", mode)
	}

	q := NewDeleteQueue([]string{file}, DefaultDeleteOptions())
	events := collectEvents(t, q.Run(context.Background()))
	if e, ok := firstEvent[ItemError](events); ok {
		t.Fatalf("trashing a %d-character path failed: %v", len(file), e)
	}

	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("file still present after being trashed, stat err = %v", err)
	}
}
