package core

import (
	"context"
	"os"
)

// ---------------------------------------------------------------------------
// DeleteQueue
// ---------------------------------------------------------------------------

// DeleteMode selects how a path is removed.
type DeleteMode int

const (
	DeletePermanent DeleteMode = iota
	DeleteToTrash
)

// VolumeKind classifies the volume holding a path.
type VolumeKind int

const (
	VolumeUnknown VolumeKind = iota
	VolumeFixed
	VolumeRemovable
)

// VolumeProber reports the kind of the volume holding a path.
type VolumeProber func(path string) VolumeKind

// ModeForVolume maps a volume kind to a delete mode: fixed volumes go to the
// recycle bin; removable, remote and unknown volumes are deleted permanently.
func ModeForVolume(k VolumeKind) DeleteMode {
	if k == VolumeFixed {
		return DeleteToTrash
	}
	return DeletePermanent
}

// DeleteOptions carries per-queue configuration for DeleteQueue.
type DeleteOptions struct {
	// ModeFor decides per path between trashing and permanent deletion.
	// Nil means permanent (current behaviour).
	ModeFor func(path string) DeleteMode
	// Trash moves a path to the platform recycle bin. Used only when ModeFor
	// reports DeleteToTrash; when nil, such a path falls back to permanent
	// deletion (never an error).
	Trash func(path string) error
}

// DeleteQueue removes files and recursive directory trees. By default it
// deletes permanently; when DeleteOptions.ModeFor reports DeleteToTrash the
// item is sent to the platform recycle bin instead. Per-item errors never
// abort the queue. Events: ItemStarted, ItemDone, ItemError, QueueDone. No
// confirmation logic lives in core (UI owns confirmation).
type DeleteQueue struct {
	items []string
	opts  DeleteOptions
}

// NewDeleteQueue creates a queue with the given items and options.
func NewDeleteQueue(items []string, opts DeleteOptions) *DeleteQueue {
	return &DeleteQueue{items: items, opts: opts}
}

// Run starts processing the queue. It returns a read-only channel of typed
// events. The channel is closed after all items are processed (or the
// context is cancelled). Blocking sends are acceptable; the consumer must
// drain the channel.
func (q *DeleteQueue) Run(ctx context.Context) <-chan any {
	ch := make(chan any)
	go q.run(ctx, ch)
	return ch
}

func (q *DeleteQueue) run(ctx context.Context, ch chan<- any) {
	defer close(ch)

	for i, item := range q.items {
		// Check cancellation before starting each item.
		select {
		case <-ctx.Done():
			ch <- ItemSkipped{Index: i, Source: item, Dest: ""}
			continue
		default:
		}

		ch <- ItemStarted{Index: i, Source: item, Dest: ""}

		// Check existence first so nonexistent sources produce ItemError.
		if _, err := os.Stat(item); os.IsNotExist(err) {
			ch <- ItemError{Index: i, Source: item, Dest: "", Cause: err}
			continue
		}

		err := q.remove(item)
		if err != nil {
			ch <- ItemError{Index: i, Source: item, Dest: "", Cause: err}
			continue
		}

		ch <- ItemDone{Index: i, Source: item, Dest: ""}
	}

	ch <- QueueDone{Err: ctx.Err()}
}

// remove applies the configured policy to a single path: it trashes the path
// only when ModeFor selects DeleteToTrash and a Trash function is provided;
// every other case (including nil options) falls back to permanent deletion.
func (q *DeleteQueue) remove(path string) error {
	if q.opts.ModeFor != nil && q.opts.ModeFor(path) == DeleteToTrash && q.opts.Trash != nil {
		return q.opts.Trash(path)
	}
	return removePath(path)
}

// DefaultDeleteOptions returns options implementing the policy: fixed volumes
// go to the recycle bin; removable, remote and unknown volumes are deleted
// permanently.
func DefaultDeleteOptions() DeleteOptions {
	prober := newVolumeProber()
	return DeleteOptions{
		ModeFor: func(path string) DeleteMode {
			return ModeForVolume(prober(path))
		},
		Trash: trashPath,
	}
}

// DefaultDeleteMode reports the delete mode the default policy would use for
// a path (used by the UI to word its confirmation).
func DefaultDeleteMode(path string) DeleteMode {
	return ModeForVolume(newVolumeProber()(path))
}
