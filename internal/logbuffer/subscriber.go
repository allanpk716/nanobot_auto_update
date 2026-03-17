package logbuffer

import (
	"context"
)

// Subscribe subscribes to log stream and returns a read-only channel
// CONTEXT.md: Channel pattern - returns <-chan LogEntry
func (lb *LogBuffer) Subscribe() <-chan LogEntry {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Create subscriber channel (capacity 100, CONTEXT.md constraint)
	ch := make(chan LogEntry, 100)

	// Create cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	lb.subscribers[ch] = cancel

	// Start subscriber goroutine
	go lb.subscriberLoop(ctx, ch)

	return ch
}

// subscriberLoop subscriber goroutine: send history logs first, then wait for real-time logs
// CONTEXT.md: New subscriber receives all history logs first
func (lb *LogBuffer) subscriberLoop(ctx context.Context, ch chan<- LogEntry) {
	defer close(ch) // Close channel when goroutine exits

	// 1. Send history logs first (CONTEXT.md constraint)
	history := lb.GetHistory()
	for _, entry := range history {
		select {
		case ch <- entry:
			// Send successful
		case <-ctx.Done():
			// Unsubscribe called, stop sending
			return
		}
	}

	// 2. Real-time logs are sent by Write() method directly to ch
	// Here we just wait for context cancellation
	<-ctx.Done()
}

// Unsubscribe unsubscribes from log stream
// CONTEXT.md: Receives channel as handle
func (lb *LogBuffer) Unsubscribe(ch <-chan LogEntry) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Try to find the channel in the map
	// Go allows comparing channels with ==
	found := false
	for subscriber, cancel := range lb.subscribers {
		// Convert both to receive-only for comparison
		// Note: In Go, chan T and <-chan T can be compared if they refer to the same channel
		var subRO <-chan LogEntry = subscriber
		if subRO == ch {
			cancel()                      // Cancel context to stop goroutine
			delete(lb.subscribers, subscriber) // Remove from map
			found = true
			lb.logger.Debug("Unsubscribed successfully")
			break
		}
	}

	if !found {
		lb.logger.Warn("Unsubscribe called with unknown channel")
	}
}
