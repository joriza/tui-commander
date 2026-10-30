//go:build windows

package core

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

// longPathsEnabledReal reads the machine-wide LongPathsEnabled registry value
// (HKLM\SYSTEM\CurrentControlSet\Control\FileSystem). It returns false when
// the value is absent or 0, which is the stock Windows default and the only
// reason a path >260 chars is rejected by the filesystem.
//
// It is the discriminator the long-path tests use to decide between skip and
// fail: a path that is already known to exceed MAX_PATH cannot be the cause
// of an MkdirAll failure, so anything other than this environment flag is a
// real setup regression.
func longPathsEnabledReal() bool {
	subkey := syscall.StringToUTF16(`SYSTEM\CurrentControlSet\Control\FileSystem`)
	var key syscall.Handle
	if err := syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, &subkey[0], 0, syscall.KEY_READ, &key); err != nil {
		return false
	}
	defer syscall.RegCloseKey(key)

	name := syscall.StringToUTF16(`LongPathsEnabled`)
	var val [4]byte
	var n uint32 = uint32(len(val))
	if err := syscall.RegQueryValueEx(key, &name[0], nil, nil, &val[0], &n); err != nil {
		return false
	}
	return val[0] == 1
}

// longPathsEnabled is the cached result of reading the machine. It is a
// package variable rather than a const so the test file can override it and
// prove the skip-vs-fail branch without touching the registry.
var longPathsEnabled = longPathsEnabledReal()

// skipLongPathFailure decides whether an MkdirAll error on a path already
// known to exceed MAX_PATH should skip the test (the machine does not
// support long paths) or fail it (the machine does support them and the
// error is something else). The path length is NOT the discriminator — it is
// guaranteed by the Fatalf above — the environment is.
func skipLongPathFailure(enabled bool) bool {
	return !enabled
}

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
		// On stock Windows the machine-wide LongPathsEnabled value defaults to 0.
		// When it is disabled the filesystem rejects paths >260 chars and the
		// test cannot run. Skip instead of failing hard so the project's green gate
		// remains usable on a vanilla machine.
		//
		// The skip is NARROW: it fires only when the machine genuinely does not
		// support long paths. A failure that is NOT that (access denied, read-only
		// temp, antivirus lock, disk full) would otherwise be swallowed and
		// reported as "ok (skipped)" forever, masking a real setup regression.
		// The path length is already guaranteed by the Fatalf above, so the
		// discriminator here is the environment, not the length.
		if skipLongPathFailure(longPathsEnabled) {
			t.Skipf("Long paths not supported on this machine (LongPathsEnabled=0; MkdirAll on %d-char path failed: %v); set LongPathsEnabled=1 in HKLM\\SYSTEM\\CurrentControlSet\\Control\\FileSystem and rerun on Windows", len(dir), err)
		}
		t.Fatalf("MkdirAll on a %d-char path failed on a machine that supports long paths: %v — this is a setup problem, not a long-path limitation", len(dir), err)
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

// TestLongPathSkipDiscriminator proves the skip-vs-fail branch actually
// discriminates the environment from a setup failure. A path already known to
// exceed MAX_PATH cannot be the cause of an MkdirAll error, so on a machine
// that supports long paths the failure must FAIL the suite (it is access
// denied, a read-only temp, an antivirus lock or a disk full), and on a
// machine that does not it must SKIP.
func TestLongPathSkipDiscriminator(t *testing.T) {
	orig := longPathsEnabled
	defer func() { longPathsEnabled = orig }()

	longPathsEnabled = true
	if skipLongPathFailure(longPathsEnabled) {
		t.Fatal("on a machine that supports long paths, a setup failure must NOT be skipped — it must fail the suite")
	}

	longPathsEnabled = false
	if !skipLongPathFailure(longPathsEnabled) {
		t.Fatal("on a machine that does not support long paths, a long-path rejection must be skipped, not failed")
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
