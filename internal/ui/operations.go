package ui

import (
	"context"
	"fmt"

	"tui-commander/internal/core"
)

// OperationRunner manages a long-running queue operation with context
// cancellation. It drains the event channel, marshals UI updates via
// QueueUpdateDraw, and reports the final state. It is generic over the
// event channel (<-chan any) and reusable by T6b (copy/move).
type OperationRunner struct {
	ctx        context.Context
	cancel     context.CancelFunc
	eventCh    <-chan any
	onDone     func(result OperationResult)
	progressCb func(done, total int)
}

// OperationResult holds the outcome of an operation.
type OperationResult struct {
	Errors    []error
	Cancelled bool
}

// NewOperationRunner creates a runner for the given event channel.
// The caller is responsible for calling Cancel() when the operation
// should be stopped.
func NewOperationRunner(eventCh <-chan any) *OperationRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &OperationRunner{
		ctx:     ctx,
		cancel:  cancel,
		eventCh: eventCh,
	}
}

// Cancel cancels the operation.
func (r *OperationRunner) Cancel() {
	r.cancel()
}

// OnDone sets the completion callback.
func (r *OperationRunner) OnDone(fn func(result OperationResult)) *OperationRunner {
	r.onDone = fn
	return r
}

// OnProgress sets the progress callback.
func (r *OperationRunner) OnProgress(fn func(done, total int)) *OperationRunner {
	r.progressCb = fn
	return r
}

// Run starts processing events in a goroutine.
func (r *OperationRunner) Run(app *App) {
	go r.run(app)
}

func (r *OperationRunner) run(app *App) {
	var result OperationResult
	var total int
	done := 0

	for event := range r.eventCh {
		switch e := event.(type) {
		case core.ItemStarted:
			total++
		case core.ItemDone:
			done++
		case core.ItemError:
			// Keep the path in the message: bare causes are useless when
			// several items fail.
			result.Errors = append(result.Errors,
				fmt.Errorf("%s: %w", e.Source, e.Cause))
			done++
		case core.ItemSkipped:
			result.Cancelled = true
			done++
		case core.QueueDone:
			if r.onDone != nil {
				app.tviewApp.QueueUpdateDraw(func() {
					r.onDone(result)
				})
			}
			return
		}

		if r.progressCb != nil {
			app.tviewApp.QueueUpdateDraw(func() {
				r.progressCb(done, total)
			})
		}
	}
}
