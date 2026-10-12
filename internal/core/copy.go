package core

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Constants and defaults
// ---------------------------------------------------------------------------

const (
	defaultBufferBytes = 1 * 1024 * 1024 // 1 MiB
	minBufferBytes     = 64 * 1024       // 64 KiB
	maxBufferBytes     = 8 * 1024 * 1024 // 8 MiB
)

// DefaultProgressMinInterval is the default maximum interval between
// progress events.
const DefaultProgressMinInterval = 100 * time.Millisecond

// DefaultProgressMinBytes is the default minimum byte increment between
// progress events. The exact throttle rule: a progress event is emitted
// when either ProgressMinInterval has elapsed since the last event OR
// ProgressMinBytes have been copied since the last event, whichever
// comes first.
const DefaultProgressMinBytes = 8 * 1024 * 1024 // 8 MiB

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

// CopyItem describes a single copy operation: source path → destination path.
// Source may be a file or a directory (recursive copy of the tree).
type CopyItem struct {
	Source string
	Dest   string
}

// CopyOptions carries per-queue configuration.
// BufferBytes is clamped to [64 KiB, 8 MiB]; zero means default (1 MiB).
// ProgressMinInterval and ProgressMinBytes control throttling: a progress
// event is emitted when either interval elapses or ProgressMinBytes are
// copied since the last event, whichever comes first. Zero values select
// the defaults (100 ms / 8 MiB).
type CopyOptions struct {
	Overwrite           bool
	BufferBytes         int
	ProgressMinInterval time.Duration
	ProgressMinBytes    int64
}

// QueueTotals is emitted once before item processing begins, carrying the
// pre-walked totals of the entire queue.
type QueueTotals struct {
	TotalBytes int64
	TotalItems int
}

// ItemStarted is emitted when an item begins processing.
type ItemStarted struct {
	Index  int
	Source string
	Dest   string
	Size   int64 // total bytes for this item (directories: total tree bytes)
}

// ItemProgress is emitted during item processing. BytesSoFar is the
// cumulative bytes copied across all items so far; Index is the queue
// index of the item currently being processed.
type ItemProgress struct {
	Index      int
	BytesSoFar int64
}

// ItemDone is emitted when an item completes successfully.
type ItemDone struct {
	Index  int
	Source string
	Dest   string
}

// ItemError is emitted when an item fails. Cause is the underlying error.
type ItemError struct {
	Index  int
	Source string
	Dest   string
	Cause  error
}

// ItemSkipped is emitted for items that were not processed because the
// queue was cancelled.
type ItemSkipped struct {
	Index  int
	Source string
	Dest   string
}

// QueueDone is emitted when the queue finishes. Err is non-nil when the
// queue was cancelled (context error).
type QueueDone struct {
	Err error
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// ErrDestExists is returned when a destination file already exists and
// Overwrite is false.
var ErrDestExists = errors.New("destination exists")

// ErrDestInsideSource is returned when the destination path is inside the
// source's own directory tree (infinite-recursion guard).
var ErrDestInsideSource = errors.New("destination is inside the source directory tree")

// ---------------------------------------------------------------------------
// CopyQueue
// ---------------------------------------------------------------------------

// CopyQueue processes a slice of CopyItem sequentially in order.
// Events are delivered over a single channel; the consumer must drain it.
type CopyQueue struct {
	items []CopyItem
	opts  CopyOptions
}

// NewCopyQueue creates a queue with the given items and options.
func NewCopyQueue(items []CopyItem, opts CopyOptions) *CopyQueue {
	return &CopyQueue{items: items, opts: opts}
}

// Run starts processing the queue. It returns a read-only channel of typed
// events. The channel is closed after all items are processed (or the
// context is cancelled). Blocking sends are acceptable; the consumer must
// drain the channel.
func (q *CopyQueue) Run(ctx context.Context) <-chan any {
	ch := make(chan any)
	go q.run(ctx, ch)
	return ch
}

func (q *CopyQueue) run(ctx context.Context, ch chan<- any) {
	defer close(ch)

	// Pre-walk sources to compute totals.
	totalBytes, totalItems, sizes := q.preWalk()
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

		ch <- ItemStarted{Index: i, Source: item.Source, Dest: item.Dest, Size: sizes[i]}

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

		// Perform the copy.
		itemStartBytes := copiedBytes
		err = q.copyItem(ctx, item.Source, item.Dest, func(n int64) {
			copiedBytes = itemStartBytes + n
			ch <- ItemProgress{Index: i, BytesSoFar: copiedBytes}
		})
		if err != nil {
			ch <- ItemError{Index: i, Source: item.Source, Dest: item.Dest, Cause: err}
			continue
		}

		ch <- ItemDone{Index: i, Source: item.Source, Dest: item.Dest}
	}

	ch <- QueueDone{Err: ctx.Err()}
}

// preWalk walks all source paths to compute total bytes and total items.
// Non-existent sources are skipped in the byte count but still counted as items.
func (q *CopyQueue) preWalk() (int64, int, []int64) {
	var totalBytes int64
	totalItems := len(q.items)
	sizes := make([]int64, totalItems)

	for i, item := range q.items {
		info, err := os.Stat(item.Source)
		if err != nil {
			continue // source doesn't exist; skip byte count
		}
		if info.IsDir() {
			var dirBytes int64
			filepath.Walk(item.Source, func(path string, fi os.FileInfo, err error) error {
				if err != nil {
					return nil // skip unreadable entries
				}
				if !fi.IsDir() {
					dirBytes += fi.Size()
				}
				return nil
			})
			sizes[i] = dirBytes
		} else {
			sizes[i] = info.Size()
		}
		totalBytes += sizes[i]
	}

	return totalBytes, totalItems, sizes
}

// copyItem copies a single item (file or directory) from src to dst.
func (q *CopyQueue) copyItem(ctx context.Context, src, dst string, onProgress func(int64)) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return q.copyDir(ctx, src, dst, onProgress)
	}
	return q.copyFile(ctx, src, dst, onProgress)
}

// copyFile copies a single file using chunked buffered I/O.
// Cancellation is checked between chunks. mtime and mode are preserved.
func (q *CopyQueue) copyFile(ctx context.Context, src, dst string, onProgress func(int64)) error {
	// Check overwrite policy.
	if _, err := os.Stat(dst); err == nil {
		if !q.opts.Overwrite {
			return ErrDestExists
		}
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	buf := q.buffer()
	var written int64
	lastEmit := time.Now()
	lastEmitBytes := int64(0)

	defer func() {
		out.Close()
		// Preserve mtime (required for files).
		stat, err := in.Stat()
		if err == nil {
			os.Chtimes(dst, stat.ModTime(), stat.ModTime())
			os.Chmod(dst, stat.Mode()) // best-effort
		}
	}()

	for {
		nr, rerr := in.Read(buf)
		if nr > 0 {
			nw, werr := out.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if werr != nil {
				os.Remove(dst)
				return werr
			}
			if nr != nw {
				os.Remove(dst)
				return io.ErrShortWrite
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			os.Remove(dst)
			return rerr
		}

		// Check cancellation between chunks.
		select {
		case <-ctx.Done():
			os.Remove(dst)
			return ctx.Err()
		default:
		}

		// Throttled progress: emit when enough time or bytes have passed.
		if shouldEmitProgress(written, lastEmitBytes, q.progressMinInterval(), q.progressMinBytes(), lastEmit) {
			onProgress(written)
			lastEmit = time.Now()
			lastEmitBytes = written
		}
	}

	// Final progress emission.
	onProgress(written)

	return nil
}

// shouldEmitProgress reports whether a progress event should be emitted
// based on the throttle settings.
func shouldEmitProgress(written, lastEmitBytes int64, minInterval time.Duration, minBytes int64, lastEmit time.Time) bool {
	if written-lastEmitBytes >= minBytes {
		return true
	}
	if time.Since(lastEmit) >= minInterval {
		return true
	}
	return false
}

// copyDir recursively copies a directory tree.
// Directory mtime is preserved best-effort after all contents are copied.
func (q *CopyQueue) copyDir(ctx context.Context, src, dst string, onProgress func(int64)) error {
	// Create destination directory.
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// Preserve source directory metadata for best-effort restoration later.
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	srcModTime := srcInfo.ModTime()
	srcMode := srcInfo.Mode()

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Check cancellation between items.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if entry.IsDir() {
			if err := q.copyDir(ctx, srcPath, dstPath, onProgress); err != nil {
				return err
			}
		} else {
			if err := q.copyFile(ctx, srcPath, dstPath, onProgress); err != nil {
				return err
			}
		}
	}

	// Best-effort directory mtime preservation (after contents).
	os.Chtimes(dst, srcModTime, srcModTime)
	os.Chmod(dst, srcMode) // best-effort

	return nil
}

// buffer returns the configured copy buffer, clamped to [minBufferBytes, maxBufferBytes].
func (q *CopyQueue) buffer() []byte {
	n := q.opts.BufferBytes
	if n <= 0 {
		n = defaultBufferBytes
	}
	if n < minBufferBytes {
		n = minBufferBytes
	}
	if n > maxBufferBytes {
		n = maxBufferBytes
	}
	return make([]byte, n)
}

// progressMinInterval returns the configured progress throttle interval,
// defaulting to DefaultProgressMinInterval if zero.
func (q *CopyQueue) progressMinInterval() time.Duration {
	d := q.opts.ProgressMinInterval
	if d <= 0 {
		return DefaultProgressMinInterval
	}
	return d
}

// progressMinBytes returns the configured progress throttle byte threshold,
// defaulting to DefaultProgressMinBytes if zero.
func (q *CopyQueue) progressMinBytes() int64 {
	n := q.opts.ProgressMinBytes
	if n <= 0 {
		return DefaultProgressMinBytes
	}
	return n
}

// isInsideSource reports whether dest is inside the source directory tree
// (infinite-recursion guard). Both paths must be cleaned absolute paths.
func isInsideSource(source, dest string) bool {
	if source == dest {
		return true
	}
	return strings.HasPrefix(dest, source+string(filepath.Separator))
}
