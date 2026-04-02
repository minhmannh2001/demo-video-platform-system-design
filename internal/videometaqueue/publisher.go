package videometaqueue

import "context"

// Publisher sends metadata-change notifications (typically to SQS).
type Publisher interface {
	Publish(ctx context.Context, ev Event) error
}

// Noop drops all events (queue disabled).
type Noop struct{}

func (Noop) Publish(context.Context, Event) error { return nil }
