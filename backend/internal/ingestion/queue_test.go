package ingestion

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/freezxp/syslogx/backend/internal/domain"
)

func TestQueueBoundsAndRelease(t *testing.T) {
	q := newFrameQueue(1, 4)
	frame := domain.Frame{Payload: []byte("1234")}
	if err := q.enqueue(context.Background(), frame, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := q.enqueue(context.Background(), domain.Frame{Payload: []byte("x")}, time.Millisecond); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected full, got %v", err)
	}
	if _, err := q.dequeue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := q.enqueue(context.Background(), domain.Frame{Payload: []byte("x")}, time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

func TestQueueCloseDrains(t *testing.T) {
	q := newFrameQueue(1, 10)
	_ = q.enqueue(context.Background(), domain.Frame{Payload: []byte("x")}, time.Millisecond)
	q.close()
	if _, err := q.dequeue(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := q.dequeue(context.Background()); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("expected closed, got %v", err)
	}
}
