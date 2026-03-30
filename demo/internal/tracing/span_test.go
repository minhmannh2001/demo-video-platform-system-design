package tracing

import (
	"context"
	"errors"
	"testing"
)

func TestFinish_nilError(t *testing.T) {
	_, span := Tracer().Start(context.Background(), "test-span")
	Finish(span, nil)
}

func TestFinish_withError(t *testing.T) {
	_, span := Tracer().Start(context.Background(), "test-span")
	Finish(span, errors.New("boom"))
}
