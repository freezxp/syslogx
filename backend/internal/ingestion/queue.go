package ingestion

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/freezxp/syslogx/backend/internal/domain"
)

var (
	ErrQueueClosed = errors.New("ingestion queue closed")
	ErrQueueFull   = errors.New("ingestion queue full")
)

type queuedFrame struct {
	frame domain.Frame
	bytes int64
}

type frameQueue struct {
	mu        sync.Mutex
	items     []queuedFrame
	head      int
	bytes     int64
	maxEvents int
	maxBytes  int64
	closed    bool
	changed   chan struct{}
}

func newFrameQueue(maxEvents int, maxBytes int64) *frameQueue {
	return &frameQueue{maxEvents: maxEvents, maxBytes: maxBytes, changed: make(chan struct{})}
}

func (q *frameQueue) notifyLocked() { close(q.changed); q.changed = make(chan struct{}) }

func (q *frameQueue) enqueue(ctx context.Context, frame domain.Frame, timeout time.Duration) error {
	size := int64(len(frame.Payload))
	if size > q.maxBytes {
		return ErrQueueFull
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			return ErrQueueClosed
		}
		if len(q.items)-q.head < q.maxEvents && q.bytes+size <= q.maxBytes {
			q.items = append(q.items, queuedFrame{frame: frame, bytes: size})
			q.bytes += size
			q.notifyLocked()
			q.mu.Unlock()
			return nil
		}
		changed := q.changed
		q.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return ErrQueueFull
		case <-changed:
		}
	}
}

func (q *frameQueue) dequeue(ctx context.Context) (queuedFrame, error) {
	for {
		q.mu.Lock()
		if q.head < len(q.items) {
			item := q.items[q.head]
			q.items[q.head] = queuedFrame{}
			q.head++
			q.bytes -= item.bytes
			if q.head > 1024 && q.head*2 >= len(q.items) {
				q.items = append([]queuedFrame(nil), q.items[q.head:]...)
				q.head = 0
			}
			q.notifyLocked()
			q.mu.Unlock()
			return item, nil
		}
		if q.closed {
			q.mu.Unlock()
			return queuedFrame{}, ErrQueueClosed
		}
		changed := q.changed
		q.mu.Unlock()
		select {
		case <-ctx.Done():
			return queuedFrame{}, ctx.Err()
		case <-changed:
		}
	}
}

func (q *frameQueue) close() {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		q.notifyLocked()
	}
	q.mu.Unlock()
}

func (q *frameQueue) usage() (events int, bytes int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items) - q.head, q.bytes
}
