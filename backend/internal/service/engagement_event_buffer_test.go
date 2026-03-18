package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/fastcrm/backend/internal/entity"
)

// makeEvent returns a minimal TrackingEvent for testing.
func makeEvent(eventType string) entity.TrackingEvent {
	return entity.TrackingEvent{
		ID:              "evt-" + eventType,
		OrgID:           "org-1",
		EnrollmentID:    "enroll-1",
		StepExecutionID: "step-exec-1",
		EventType:       eventType,
		OccurredAt:      time.Now(),
		CreatedAt:       time.Now(),
	}
}

// TestEventBuffer_EnqueueSuccess verifies that enqueueing to a non-full buffer
// returns true without blocking.
func TestEventBuffer_EnqueueSuccess(t *testing.T) {
	buf := NewEventBuffer(5)
	ok := buf.Enqueue(makeEvent(entity.TrackingEventOpen))
	if !ok {
		t.Fatal("Enqueue returned false on a non-full buffer — expected true")
	}
}

// TestEventBuffer_EnqueueDropsWhenFull verifies that enqueueing to a full buffer
// returns false immediately (does not block).
func TestEventBuffer_EnqueueDropsWhenFull(t *testing.T) {
	const bufSize = 5
	buf := NewEventBuffer(bufSize)

	// Fill the buffer to capacity.
	for i := 0; i < bufSize; i++ {
		ok := buf.Enqueue(makeEvent(entity.TrackingEventOpen))
		if !ok {
			t.Fatalf("Enqueue %d returned false before buffer was full", i+1)
		}
	}

	// Next enqueue must drop without blocking.
	done := make(chan bool, 1)
	go func() {
		ok := buf.Enqueue(makeEvent(entity.TrackingEventClick))
		done <- ok
	}()

	select {
	case result := <-done:
		if result {
			t.Fatal("Enqueue returned true when buffer was full — expected false")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Enqueue blocked when buffer was full — must return immediately")
	}
}

// TestEventBuffer_FlushDrainsBatch verifies that events enqueued before a flush
// tick are delivered to the registered flushFn as a batch.
func TestEventBuffer_FlushDrainsBatch(t *testing.T) {
	const flushInterval = 50 * time.Millisecond
	buf := NewEventBuffer(5)

	var mu sync.Mutex
	var received []entity.TrackingEvent
	flushed := make(chan struct{}, 1)

	buf.SetFlushFunc(func(batch []entity.TrackingEvent) {
		mu.Lock()
		received = append(received, batch...)
		mu.Unlock()
		select {
		case flushed <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	buf.Start(ctx, flushInterval)

	// Enqueue 3 events before the first flush tick.
	buf.Enqueue(makeEvent(entity.TrackingEventOpen))
	buf.Enqueue(makeEvent(entity.TrackingEventClick))
	buf.Enqueue(makeEvent(entity.TrackingEventReply))

	// Wait for at least one flush to occur (generous timeout).
	select {
	case <-flushed:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("flushFn was not called within 500ms")
	}

	buf.Stop()

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 3 {
		t.Fatalf("Expected 3 events in batch, got %d", count)
	}
}

// TestEventBuffer_StopCleansUp verifies that calling Stop halts the flush loop
// and that no further flushFn calls occur after Stop returns.
func TestEventBuffer_StopCleansUp(t *testing.T) {
	const flushInterval = 50 * time.Millisecond
	buf := NewEventBuffer(5)

	var callCount int
	var mu sync.Mutex

	buf.SetFlushFunc(func(batch []entity.TrackingEvent) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	ctx := context.Background()
	buf.Start(ctx, flushInterval)

	// Give the loop a moment to initialise, then stop.
	time.Sleep(20 * time.Millisecond)
	buf.Stop()

	mu.Lock()
	countAfterStop := callCount
	mu.Unlock()

	// Wait two flush intervals to confirm no additional calls.
	time.Sleep(2 * flushInterval)

	mu.Lock()
	countFinal := callCount
	mu.Unlock()

	if countFinal != countAfterStop {
		t.Fatalf("flushFn was called after Stop: calls before=%d after=%d", countAfterStop, countFinal)
	}
}

// TestEventBuffer_DoubleStartIsNoop verifies that calling Start twice does not
// create duplicate goroutines (the second call must be silently ignored).
func TestEventBuffer_DoubleStartIsNoop(t *testing.T) {
	const flushInterval = 50 * time.Millisecond
	buf := NewEventBuffer(5)

	var callCount int
	var mu sync.Mutex

	buf.SetFlushFunc(func(batch []entity.TrackingEvent) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start twice — second call must be a no-op.
	buf.Start(ctx, flushInterval)
	buf.Start(ctx, flushInterval)

	// Enqueue one event.
	buf.Enqueue(makeEvent(entity.TrackingEventOpen))

	// Allow enough time for two flush intervals.
	time.Sleep(3 * flushInterval)
	buf.Stop()

	mu.Lock()
	count := callCount
	mu.Unlock()

	// With a single goroutine the flush function should be called a small
	// number of times (1–3), not doubled. Anything over 10 in this window
	// would indicate duplicate goroutines.
	if count > 10 {
		t.Fatalf("flushFn called %d times — suggests duplicate goroutines from double Start", count)
	}
}
