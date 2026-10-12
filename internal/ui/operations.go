package ui

import (
	"context"
	"fmt"
	"math"
	"time"

	"tui-commander/internal/core"
)

// ProgressETA holds ETA and throughput data computed by the
// OperationRunner from progress event timestamps.
type ProgressETA struct {
	RateMBs   float64 // smoothed throughput in MB/s; 0 when unknown
	Remaining string  // human-readable remaining time, e.g. "1m23s" or "Calculating..."
}

// OperationRunner manages a long-running queue operation with context
// cancellation. It drains the event channel, marshals UI updates via
// QueueUpdateDraw, and reports the final state. It is generic over the
// event channel (<-chan any) and reusable by T6b (copy/move).
type OperationRunner struct {
	ctx             context.Context
	cancel          context.CancelFunc
	eventCh         <-chan any
	onDone          func(result OperationResult)
	progressCb      func(currentName string, fileBytes, fileSize int64, done, total int, doneBytes, totalBytes int64, eta ProgressETA)
	totalBytes      int64
	doneBytes       int64
	currentName     string
	currentFileSize int64
	currentFileBytes int64
	// EMA state for smoothed rate.
	prevRate      float64
	prevDoneBytes int64
	prevTime      time.Time
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
func (r *OperationRunner) OnProgress(fn func(currentName string, fileBytes, fileSize int64, done, total int, doneBytes, totalBytes int64, eta ProgressETA)) *OperationRunner {
	r.progressCb = fn
	return r
}

// Run starts processing events in a goroutine.
func (r *OperationRunner) Run(app *App) {
	go r.run(app)
}

// alpha is the EMA smoothing factor for throughput.
const alpha = 0.3

func (r *OperationRunner) run(app *App) {
	var result OperationResult
	var total int
	done := 0

	for event := range r.eventCh {
		switch e := event.(type) {
		case core.QueueTotals:
			r.totalBytes = e.TotalBytes
			total = e.TotalItems
		case core.ItemStarted:
			total++
			r.currentName = e.Source
			r.currentFileSize = e.Size
			r.currentFileBytes = 0
		case core.ItemProgress:
			r.doneBytes = e.BytesSoFar
			r.currentFileBytes = e.BytesSoFar
			// Update EMA rate.
			now := time.Now()
			if r.prevDoneBytes > 0 && r.prevTime.IsZero() {
				r.prevTime = now
			}
			if r.prevDoneBytes > 0 && now.Sub(r.prevTime) > 0 {
				instantRate := float64(e.BytesSoFar-r.prevDoneBytes) / now.Sub(r.prevTime).Seconds() / (1024 * 1024)
				if r.prevRate == 0 {
					r.prevRate = instantRate
				} else {
					r.prevRate = alpha*instantRate + (1-alpha)*r.prevRate
				}
			}
			r.prevDoneBytes = e.BytesSoFar
			r.prevTime = now
		case core.ItemDone:
			done++
			r.currentFileBytes = r.currentFileSize
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
			eta := r.computeETA()
			app.tviewApp.QueueUpdateDraw(func() {
				r.progressCb(r.currentName, r.currentFileBytes, r.currentFileSize, done, total, r.doneBytes, r.totalBytes, eta)
			})
		}
	}
}

// computeETA calculates the ETA and smoothed throughput.
func (r *OperationRunner) computeETA() ProgressETA {
	if r.totalBytes <= 0 || r.doneBytes <= 0 {
		return ProgressETA{Remaining: "Calculating..."}
	}
	// Show ETA only once we've passed 2%.
	if float64(r.doneBytes)/float64(r.totalBytes) < 0.02 {
		return ProgressETA{Remaining: "Calculating..."}
	}
	rate := r.prevRate
	if rate <= 0 {
		return ProgressETA{Remaining: "Calculating..."}
	}
	remainingBytes := r.totalBytes - r.doneBytes
	remainingSecs := float64(remainingBytes) / (rate * 1024 * 1024)
	return ProgressETA{
		RateMBs:   math.Round(rate*100) / 100,
		Remaining: formatETA(remainingSecs),
	}
}

// formatETA formats a duration in seconds as "1m23s" or "2h05m".
func formatETA(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalSecs := int(seconds)
	hours := totalSecs / 3600
	mins := (totalSecs % 3600) / 60
	secs := totalSecs % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, mins)
	}
	return fmt.Sprintf("%dm%ds", mins, secs)
}
