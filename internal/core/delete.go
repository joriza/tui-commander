package core

import (
	"context"
	"os"
)

// ---------------------------------------------------------------------------
// DeleteQueue
// ---------------------------------------------------------------------------

// DeleteOptions carries per-queue configuration for DeleteQueue.
type DeleteOptions struct{}

// DeleteQueue permanently deletes files and recursive directory trees.
// Per-item errors never abort the queue. Events: ItemStarted, ItemDone,
// ItemError, QueueDone. No confirmation logic lives in core (UI owns
// confirmation).
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

		err := removePath(item)
		if err != nil {
			ch <- ItemError{Index: i, Source: item, Dest: "", Cause: err}
			continue
		}

		ch <- ItemDone{Index: i, Source: item, Dest: ""}
	}

	ch <- QueueDone{Err: ctx.Err()}
}
