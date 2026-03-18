package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/fastcrm/backend/internal/entity"
)

// Tunable defaults — adjust via NewEventBuffer / Start parameters.
const (
	DefaultEventBufferSize    = 1000          // Channel capacity
	DefaultEventFlushInterval = 5 * time.Second
)

// EventBuffer is a non-blocking buffered channel for high-write email tracking
// events. Callers enqueue events without waiting for a DB write; a background
// goroutine periodically drains the channel and writes batches to the database.
//
// The flush function is intentionally pluggable (nil during Phase 32) so that
// Phase 35 can wire in actual DB persistence without touching this scaffold.
type EventBuffer struct {
	ch      chan entity.TrackingEvent
	stopCh  chan struct{}
	flushFn func([]entity.TrackingEvent) // nil → events are discarded (Phase 32)
	mu      sync.Mutex
	running bool
}

// NewEventBuffer creates an EventBuffer with the given channel capacity.
// Use DefaultEventBufferSize unless benchmarks indicate otherwise.
func NewEventBuffer(bufSize int) *EventBuffer {
	return &EventBuffer{
		ch:     make(chan entity.TrackingEvent, bufSize),
		stopCh: make(chan struct{}),
	}
}

// SetFlushFunc registers the function that receives each drained batch.
// It replaces any previously registered function and is safe to call at any time.
func (b *EventBuffer) SetFlushFunc(fn func([]entity.TrackingEvent)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushFn = fn
}

// Enqueue adds an event to the buffer without blocking the caller.
// Returns true if the event was queued, false if the buffer was full (event
// is dropped and a warning is logged).
func (b *EventBuffer) Enqueue(e entity.TrackingEvent) bool {
	select {
	case b.ch <- e:
		return true
	default:
		log.Printf("[EVENT-BUFFER] Buffer full, dropping %s event for enrollment %s", e.EventType, e.EnrollmentID)
		return false
	}
}

// Start begins the background flush loop. Calling Start on an already-running
// buffer is a no-op (second call is silently ignored).
func (b *EventBuffer) Start(ctx context.Context, flushInterval time.Duration) {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return
	}
	b.running = true
	b.mu.Unlock()
	go b.flushLoop(ctx, flushInterval)
}

// Stop signals the flush loop to drain remaining events and exit.
// Calling Stop on a buffer that is not running is a no-op.
func (b *EventBuffer) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.running {
		return
	}
	close(b.stopCh)
	b.running = false
}

// flushLoop runs in a goroutine started by Start. It wakes on each ticker tick
// (or on context cancellation / Stop signal) and drains all pending events into
// a batch slice before calling flushFn.
func (b *EventBuffer) flushLoop(ctx context.Context, flushInterval time.Duration) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.drainAndFlush()

		case <-ctx.Done():
			log.Printf("[EVENT-BUFFER] Context cancelled — flushing remaining events")
			b.drainAndFlush()
			return

		case <-b.stopCh:
			log.Printf("[EVENT-BUFFER] Stop signal received — flushing remaining events")
			b.drainAndFlush()
			return
		}
	}
}

// drainAndFlush collects all events currently in the channel (non-blocking)
// and calls the registered flushFn if the batch is non-empty.
func (b *EventBuffer) drainAndFlush() {
	var batch []entity.TrackingEvent
	for {
		select {
		case e := <-b.ch:
			batch = append(batch, e)
		default:
			// Channel empty — stop draining
			goto done
		}
	}
done:
	if len(batch) == 0 {
		return
	}
	log.Printf("[EVENT-BUFFER] Flushing batch of %d events", len(batch))

	b.mu.Lock()
	fn := b.flushFn
	b.mu.Unlock()

	if fn != nil {
		fn(batch)
	}
}
