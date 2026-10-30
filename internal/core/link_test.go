package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestLoad_MarksSymlinkEntry records that a directory entry which is a symlink
// is marked in the listing without the listing following it. The entry keeps
// its IsDir from the link target (so it sorts and renders like a directory)
// while Link carries the resolved target for the `name -> target` marker.
func TestLoad_MarksSymlinkEntry(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-dir")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "link-to-real")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("Symlink not supported on this platform: %v", err)
	}

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	var found Entry
	for _, e := range p.Entries() {
		if e.Name == "link-to-real" {
			found = e
			break
		}
	}
	if found.Name == "" {
		t.Fatal("symlink entry missing from the listing")
	}
	if found.Link != target {
		t.Errorf("Link = %q, want %q", found.Link, target)
	}
}

// TestLoad_DoesNotFollowSymlinkIntoListing proves the listing never descends
// through a symlink: the panel's entries are the parent directory's entries,
// not the target's. A symlink whose target is a directory with its own files
// must not leak those files into the panel.
func TestLoad_DoesNotFollowSymlinkIntoListing(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real-dir")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "inside.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "link-to-real")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("Symlink not supported on this platform: %v", err)
	}

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	names := map[string]bool{}
	for _, e := range p.Entries() {
		names[e.Name] = true
	}
	if names["link-to-real"] {
		t.Fatal("the symlink itself is present, which is correct")
	}
	if names["inside.txt"] {
		t.Fatal("the listing followed the symlink into its target: the target's entries leaked into the panel")
	}
}

// TestLoad_PlainEntryHasEmptyLink keeps the invariant that a normal entry
// carries no link target, so renderers can use Link == "" as the "not a link"
// test without special-casing the platform.
func TestLoad_PlainEntryHasEmptyLink(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, e := range p.Entries() {
		if e.Link != "" {
			t.Errorf("entry %q has Link = %q, want empty for a plain entry", e.Name, e.Link)
		}
	}
}

// TestLoad_MarksFileSymlink records that a symlink pointing at a FILE is also
// marked (it is not a directory, so it must never be descended into either).
func TestLoad_MarksFileSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "link-to-real.txt")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("Symlink not supported on this platform: %v", err)
	}

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	var found Entry
	for _, e := range p.Entries() {
		if e.Name == "link-to-real.txt" {
			found = e
			break
		}
	}
	if found.Name == "" {
		t.Fatal("file symlink entry missing from the listing")
	}
	if found.Link != target {
		t.Errorf("Link = %q, want %q", found.Link, target)
	}
}

// TestLoad_JunctionOnWindows records the same invariant for a junction (a
// reparse point that is not a symlink). The marking code is the same as for a
// symlink — os.Readlink resolves a junction's substitute name on Windows —
// so the invariant is covered by the symlink tests. This test exists to record
// that junctions are in scope and to seed one when the platform exposes a
// creator; the Go runtime exposes no public junction creator, so on stock
// tooling it skips rather than asserts.
func TestLoad_JunctionOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions are a Windows reparse-point concept")
	}
	// The marking path (os.Readlink on an entry whose Mode()&ModeSymlink is
	// set) covers junctions on Windows: Readlink uses FSCTL_GET_REPARSE_POINT
	// and returns the substitute name for both symlinks and junctions. A
	// junction cannot be created through the public os package, so we cannot
	// seed one here; the invariant is asserted by the symlink tests.
	t.Skip("the public os package creates no junctions; the junction invariant is covered by the symlink tests, which use the same os.Readlink path")
}
