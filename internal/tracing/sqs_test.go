package tracing

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestExtractFromSQSAttributes_nilCarrier(t *testing.T) {
	ctx := context.Background()
	got := ExtractFromSQSAttributes(ctx, nil)
	if got != ctx {
		t.Fatal("expected same context when no attributes")
	}
}

func TestInjectExtractSQS_roundTripTraceID(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	ctx, parent := otel.Tracer("test").Start(context.Background(), "http.send")
	attrs := InjectIntoSQSAttributes(ctx)
	parent.End()

	if len(attrs) == 0 {
		t.Fatal("expected traceparent in SQS attributes")
	}

	ctx2 := ExtractFromSQSAttributes(context.Background(), attrs)
	_, child := otel.Tracer("test").Start(ctx2, "worker.job")
	defer child.End()

	want := trace.SpanContextFromContext(ctx).TraceID()
	got := trace.SpanContextFromContext(ctx2).TraceID()
	if got != want {
		t.Fatalf("trace id mismatch: %v vs %v", got, want)
	}

	childParent := trace.SpanContextFromContext(ctx2)
	if !childParent.IsValid() {
		t.Fatal("extracted context should carry valid span context")
	}
}

func TestExtractFromSQSAttributes_skipsNonString(t *testing.T) {
	ctx := context.Background()
	attrs := map[string]sqstypes.MessageAttributeValue{
		"traceparent": {DataType: aws.String("Binary"), BinaryValue: []byte("x")},
	}
	got := ExtractFromSQSAttributes(ctx, attrs)
	if got != ctx {
		t.Fatal("non-String attributes should be ignored")
	}
}
