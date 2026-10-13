package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// TestDeleteQueue_File
// ---------------------------------------------------------------------------

func TestDeleteQueue_File(t *testing.T) {
	srcDir := t.TempDir()
	target := filepath.Join(srcDir, "to-delete.txt")
	if err := os.WriteFile(target, []byte("gone soon"), 0644); err != nil {
		t.Fatal(err)
	}

	q := NewDeleteQueue([]string{target}, DeleteOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("file should be deleted, err = %v", err)
	}

	var started ItemStarted
	if _, ok := firstEvent[ItemStarted](events); !ok {
		t.Fatal("expected ItemStarted")
	}
	_ = started

	var done ItemDone
	if _, ok := firstEvent[ItemDone](events); !ok {
		t.Fatal("expected ItemDone")
	}
	_ = done

	var qd QueueDone
	if _, ok := firstEvent[QueueDone](events); !ok {
		t.Fatal("expected QueueDone")
	}
	if qd.Err != nil {
		t.Errorf("QueueDone.Err = %v, want nil", qd.Err)
	}
}

// ---------------------------------------------------------------------------
// TestDeleteQueue_RecursiveTree
// ---------------------------------------------------------------------------

func TestDeleteQueue_RecursiveTree(t *testing.T) {
	srcDir := t.TempDir()
	target := filepath.Join(srcDir, "tree")
	mkdirAll(t, filepath.Join(target, "sub", "deep"))
	writeFile(t, target, "a.txt", []byte("file a"), 0644)
	writeFile(t, filepath.Join(target, "sub"), "b.txt", []byte("file b"), 0644)
	writeFile(t, filepath.Join(target, "sub", "deep"), "c.txt", []byte("file c"), 0644)

	q := NewDeleteQueue([]string{target}, DeleteOptions{})
	ch := q.Run(context.Background())
	collectEvents(t, ch)

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("tree should be deleted, err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestDeleteQueue_NonexistentSource
// ---------------------------------------------------------------------------

func TestDeleteQueue_NonexistentSource(t *testing.T) {
	q := NewDeleteQueue([]string{"/nonexistent/path"}, DeleteOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var errs []ItemError
	for _, e := range events {
		if v, ok := e.(ItemError); ok {
			errs = append(errs, v)
		}
	}
	if len(errs) == 0 {
		t.Fatal("expected ItemError for nonexistent source")
	}

	// Queue should still complete.
	if _, ok := firstEvent[QueueDone](events); !ok {
		t.Fatal("expected QueueDone")
	}
}

// ---------------------------------------------------------------------------
// TestDeleteQueue_PerItemErrorDoesNotAbort
// ---------------------------------------------------------------------------

func TestDeleteQueue_PerItemErrorDoesNotAbort(t *testing.T) {
	srcDir := t.TempDir()

	valid := filepath.Join(srcDir, "valid.txt")
	if err := os.WriteFile(valid, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	nonexistent := filepath.Join(srcDir, "does-not-exist.txt")

	q := NewDeleteQueue([]string{valid, nonexistent}, DeleteOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var errs []ItemError
	for _, e := range events {
		if v, ok := e.(ItemError); ok {
			errs = append(errs, v)
		}
	}
	if len(errs) == 0 {
		t.Fatal("expected at least one ItemError")
	}

	// Valid file should still be deleted.
	if _, err := os.Stat(valid); !os.IsNotExist(err) {
		t.Error("valid file should have been deleted")
	}
}

// ---------------------------------------------------------------------------
// TestDeleteQueue_EmptyQueue
// ---------------------------------------------------------------------------

func TestDeleteQueue_EmptyQueue(t *testing.T) {
	q := NewDeleteQueue(nil, DeleteOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var qd QueueDone
	if _, ok := firstEvent[QueueDone](events); !ok {
		t.Fatal("expected QueueDone")
	}
	if qd.Err != nil {
		t.Errorf("QueueDone.Err = %v, want nil", qd.Err)
	}
}

// ---------------------------------------------------------------------------
// TestModeForVolume
// ---------------------------------------------------------------------------

func TestModeForVolume(t *testing.T) {
	tests := []struct {
		name string
		kind VolumeKind
		want DeleteMode
	}{
		{"fixed volume goes to recycle bin", VolumeFixed, DeleteToTrash},
		{"removable volume is permanent", VolumeRemovable, DeletePermanent},
		{"unknown volume is permanent", VolumeUnknown, DeletePermanent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ModeForVolume(tt.kind); got != tt.want {
				t.Errorf("ModeForVolume(%v) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestDeleteQueue_TrashModeUsesTrash
// ---------------------------------------------------------------------------

func TestDeleteQueue_TrashModeUsesTrash(t *testing.T) {
	srcDir := t.TempDir()
	target := writeFile(t, srcDir, "keep.txt", []byte("keep me"), 0644)

	calls := 0
	opts := DeleteOptions{
		ModeFor: func(string) DeleteMode { return DeleteToTrash },
		Trash: func(string) error {
			calls++
			return nil
		},
	}

	q := NewDeleteQueue([]string{target}, opts)
	events := collectEvents(t, q.Run(context.Background()))

	if calls != 1 {
		t.Errorf("Trash called %d times, want 1", calls)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("file should still exist after trash, err = %v", err)
	}
	if _, ok := firstEvent[ItemDone](events); !ok {
		t.Error("expected ItemDone")
	}
}

// ---------------------------------------------------------------------------
// TestDeleteQueue_PermanentModeRemovesFile
// ---------------------------------------------------------------------------

func TestDeleteQueue_PermanentModeRemovesFile(t *testing.T) {
	srcDir := t.TempDir()
	target := writeFile(t, srcDir, "gone.txt", []byte("bye"), 0644)

	calls := 0
	opts := DeleteOptions{
		ModeFor: func(string) DeleteMode { return DeletePermanent },
		Trash: func(string) error {
			calls++
			return nil
		},
	}

	q := NewDeleteQueue([]string{target}, opts)
	collectEvents(t, q.Run(context.Background()))

	if calls != 0 {
		t.Errorf("Trash called %d times, want 0", calls)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("file should be permanently deleted, err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestDeleteQueue_TrashModeNilTrashFallsBackToPermanent
// ---------------------------------------------------------------------------

func TestDeleteQueue_TrashModeNilTrashFallsBackToPermanent(t *testing.T) {
	srcDir := t.TempDir()
	target := writeFile(t, srcDir, "fallback.txt", []byte("x"), 0644)

	opts := DeleteOptions{
		ModeFor: func(string) DeleteMode { return DeleteToTrash },
		// Trash is nil: must fall back to permanent deletion, not error.
	}

	q := NewDeleteQueue([]string{target}, opts)
	events := collectEvents(t, q.Run(context.Background()))

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("file should fall back to permanent deletion, err = %v", err)
	}
	if _, ok := firstEvent[ItemDone](events); !ok {
		t.Error("expected ItemDone")
	}
	if errs := findEvent[ItemError](events); len(errs) > 0 {
		t.Errorf("unexpected ItemError: %v", errs[0].Cause)
	}
}

// ---------------------------------------------------------------------------
// TestDeleteQueue_NilModeForIsPermanent
// ---------------------------------------------------------------------------

func TestDeleteQueue_NilModeForIsPermanent(t *testing.T) {
	srcDir := t.TempDir()
	target := writeFile(t, srcDir, "nil-mode.txt", []byte("x"), 0644)

	opts := DeleteOptions{
		Trash: func(string) error { return errors.New("Trash should not be called") },
		// ModeFor is nil: zero value keeps today's permanent-only behaviour.
	}

	q := NewDeleteQueue([]string{target}, opts)
	collectEvents(t, q.Run(context.Background()))

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("file should be permanently deleted, err = %v", err)
	}
}
