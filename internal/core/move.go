package core

import (
	"context"
	"os"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// MoveItem, MoveOptions, MoveQueue
// ---------------------------------------------------------------------------

// MoveItem describes a single move operation: source path → destination path.
// Source may be a file or a directory (recursive move of the tree).
type MoveItem struct {
	Source string
	Dest   string
}

// MoveOptions carries per-queue configuration.
// Overwrite controls whether an existing destination is replaced.
// ForceFallback is for testing only; when true the same-volume os.Rename
// fast path is bypassed and the copy-then-delete fallback is used for every
// item regardless of volume boundaries.
type MoveOptions struct {
	Overwrite     bool
	ForceFallback bool
}

// MoveQueue processes a slice of MoveItem sequentially in order.
// Events are delivered over a single channel; the consumer must drain it.
// Same-volume moves use os.Rename (fast path: no byte-progress events).
// Different-volume moves fall back to the copy engine then delete the source
// tree on per-item success.
type MoveQueue struct {
	items []MoveItem
	opts  MoveOptions
}

// NewMoveQueue creates a queue with the given items and options.
func NewMoveQueue(items []MoveItem, opts MoveOptions) *MoveQueue {
	return &MoveQueue{items: items, opts: opts}
}

// Run starts processing the queue. It returns a read-only channel of typed
// events. The channel is closed after all items are processed (or the
// context is cancelled). Blocking sends are acceptable; the consumer must
// drain the channel.
func (q *MoveQueue) Run(ctx context.Context) <-chan any {
	ch := make(chan any)
	go q.run(ctx, ch)
	return ch
}

func (q *MoveQueue) run(ctx context.Context, ch chan<- any) {
	defer close(ch)

	// Pre-walk sources to compute totals.
	totalBytes, totalItems := q.preWalk()
	ch <- QueueTotals{TotalBytes: totalBytes, TotalItems: totalItems}

	// Process items sequentially.
	var copiedBytes int64
	for i, item := range q.items {
		// Check cancellation before starting each item.
		select {
		case <-ctx.Done():
			ch <- ItemSkipped{Index: i, Source: item.Source, Dest: item.Dest}
			continue
		default:
		}

		ch <- ItemStarted{Index: i, Source: item.Source, Dest: item.Dest}

		// Resolve cleaned absolute paths for the infinite-recursion guard.
		srcAbs, err := filepath.Abs(filepath.Clean(item.Source))
		if err != nil {
			ch <- ItemError{Index: i, Source: item.Source, Dest: item.Dest, Cause: err}
			continue
		}
		dstAbs, err := filepath.Abs(filepath.Clean(item.Dest))
		if err != nil {
			ch <- ItemError{Index: i, Source: item.Source, Dest: item.Dest, Cause: err}
			continue
		}

		// Guard: destination inside source's own directory tree.
		if isInsideSource(srcAbs, dstAbs) {
			ch <- ItemError{
				Index: i, Source: item.Source, Dest: item.Dest,
				Cause: ErrDestInsideSource,
			}
			continue
		}

		// Overwrite policy check.
		if _, err := os.Stat(dstAbs); err == nil && !q.opts.Overwrite {
			ch <- ItemError{
				Index: i, Source: item.Source, Dest: item.Dest,
				Cause: ErrDestExists,
			}
			continue
		}

		// Decide path: same-volume fast rename vs copy+delete fallback.
		if !q.opts.ForceFallback {
			sv, err := sameVolume(srcAbs, dstAbs)
			if err == nil && sv {
				// Fast path: os.Rename preserves mtime natively.
				if q.opts.Overwrite {
					os.RemoveAll(dstAbs)
				}
				err = os.Rename(srcAbs, dstAbs)
				if err != nil {
					ch <- ItemError{Index: i, Source: item.Source, Dest: item.Dest, Cause: err}
					continue
				}
				ch <- ItemDone{Index: i, Source: item.Source, Dest: item.Dest}
				continue
			}
			// Cross-device or sameVolume error → fallback.
		}

		// Fallback path: copy engine then delete source tree.
		itemStartBytes := copiedBytes
		var copyErr error
		copyQ := NewCopyQueue([]CopyItem{{Source: item.Source, Dest: item.Dest}}, CopyOptions{
			Overwrite: q.opts.Overwrite,
		})
		copyCh := copyQ.Run(ctx)
		for ev := range copyCh {
			switch e := ev.(type) {
			case ItemProgress:
				copiedBytes = itemStartBytes + e.BytesSoFar
				ch <- ItemProgress{Index: i, BytesSoFar: copiedBytes}
			case ItemDone:
				// copy succeeded; fall through to source removal.
			case ItemError:
				copyErr = e.Cause
			}
		}
		if copyErr != nil {
			ch <- ItemError{Index: i, Source: item.Source, Dest: item.Dest, Cause: copyErr}
			continue
		}

		// Delete the source tree after a successful copy.
		if err := os.RemoveAll(srcAbs); err != nil {
			ch <- ItemError{Index: i, Source: item.Source, Dest: item.Dest, Cause: err}
			continue
		}

		ch <- ItemDone{Index: i, Source: item.Source, Dest: item.Dest}
	}

	ch <- QueueDone{Err: ctx.Err()}
}

// preWalk walks all source paths to compute total bytes and total items.
// Non-existent sources are skipped in the byte count but still counted as items.
func (q *MoveQueue) preWalk() (int64, int) {
	var totalBytes int64
	totalItems := len(q.items)

	for _, item := range q.items {
		info, err := os.Stat(item.Source)
		if err != nil {
			continue
		}
		if info.IsDir() {
			filepath.Walk(item.Source, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !fi.IsDir() {
					totalBytes += fi.Size()
				}
				return nil
			})
		} else {
			totalBytes += info.Size()
		}
	}

	return totalBytes, totalItems
}
