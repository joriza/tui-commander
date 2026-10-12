package core

import (
	"context"
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
