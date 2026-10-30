package ui

import (
	"strings"
	"testing"

	"tui-commander/internal/core"
)

// TestLinkSuffix_MarksSymlink renders the `name -> target` marker that the
// listing shows for a symlink or junction. It is a pure function so it can
// be asserted headlessly: the filesystem is never touched here.
func TestLinkSuffix_MarksSymlink(t *testing.T) {
	e := core.Entry{Name: "link-to-real", IsDir: true, Link: "real-dir"}
	got := linkSuffix(e)
	if !strings.Contains(got, "->") {
		t.Errorf("linkSuffix = %q, want a '->' marker", got)
	}
	if !strings.Contains(got, "real-dir") {
		t.Errorf("linkSuffix = %q, want the target 'real-dir' present", got)
	}
}

// TestLinkSuffix_MarksFileSymlink covers a link to a file, not just a
// directory: the marker is about the link, not about the target kind.
func TestLinkSuffix_MarksFileSymlink(t *testing.T) {
	e := core.Entry{Name: "link.txt", Link: "real.txt"}
	got := linkSuffix(e)
	if !strings.Contains(got, "real.txt") {
		t.Errorf("linkSuffix = %q, want the target 'real.txt' present", got)
	}
}

// TestLinkSuffix_EmptyForPlainEntry keeps the invariant that a normal entry
// carries no marker, so renderers can use the empty string as the "not a
// link" test.
func TestLinkSuffix_EmptyForPlainEntry(t *testing.T) {
	if got := linkSuffix(core.Entry{Name: "file.txt"}); got != "" {
		t.Errorf("linkSuffix = %q, want empty for a plain entry", got)
	}
	if got := linkSuffix(core.Entry{Name: "dir", IsDir: true}); got != "" {
		t.Errorf("linkSuffix = %q, want empty for a plain directory", got)
	}
}

// TestLinkSuffix_DanglingLink reports the stored target even when the link
// points at something that no longer exists: the listing is honest about
// what the link says, and the traversal policy decides what to do.
func TestLinkSuffix_DanglingLink(t *testing.T) {
	e := core.Entry{Name: "dangling", Link: "gone-target"}
	got := linkSuffix(e)
	if !strings.Contains(got, "gone-target") {
		t.Errorf("linkSuffix = %q, want the stored target 'gone-target' present", got)
	}
}

// TestLinkSuffix_TruncatesLongTarget keeps the marker from overflowing the
// column: a very long target is shortened in the middle, preserving start and
// end, instead of pushing the row past the panel width.
func TestLinkSuffix_TruncatesLongTarget(t *testing.T) {
	long := strings.Repeat("a", 200) + ".txt"
	e := core.Entry{Name: "link", Link: long}
	got := linkSuffix(e)
	if len(got) > 60 {
		t.Errorf("linkSuffix = %d chars, want it truncated well under 60", len(got))
	}
	if !strings.HasSuffix(got, ".txt") {
		t.Errorf("linkSuffix = %q, want the extension preserved", got)
	}
}
