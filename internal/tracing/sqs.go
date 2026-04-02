package tracing

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// InjectIntoSQSAttributes serializes the trace context from ctx into SQS message
// attributes (W3C traceparent / tracestate) for distributed tracing across the queue.
func InjectIntoSQSAttributes(ctx context.Context) map[string]sqstypes.MessageAttributeValue {
	carrier := make(map[string]string)
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(carrier))
	if len(carrier) == 0 {
		return nil
	}
	out := make(map[string]sqstypes.MessageAttributeValue, len(carrier))
	for k, v := range carrier {
		out[k] = sqstypes.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(v),
		}
	}
	return out
}

// ExtractFromSQSAttributes restores trace context from SQS message attributes (if present).
func ExtractFromSQSAttributes(ctx context.Context, attrs map[string]sqstypes.MessageAttributeValue) context.Context {
	if len(attrs) == 0 {
		return ctx
	}
	carrier := make(map[string]string)
	for k, v := range attrs {
		if v.StringValue != nil && aws.ToString(v.DataType) == "String" {
			carrier[k] = aws.ToString(v.StringValue)
		}
	}
	if len(carrier) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(carrier))
}
