package wsevents

import (
	"context"
	"encoding/json"
	"log/slog"

	"video-platform/demo/internal/tracing"
	"video-platform/demo/internal/ws"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
)

// redisEnvelope is the cross-process message on Redis Pub/Sub.
type redisEnvelope struct {
	Topic string          `json:"topic"`
	Body  json.RawMessage `json:"body"`
}

// Bridge routes realtime events: local Hub and/or Redis Pub/Sub (multi-instance).
//
// When channel is non-empty and rdb is set, Publish writes only to Redis; RunSubscriber
// on the API fans in to the Hub (avoids duplicate delivery on the same pod).
//
// When channel is empty, Publish writes only to the Hub (API process only; worker cannot reach WS).
type Bridge struct {
	hub     *ws.Hub
	rdb     *redis.Client
	channel string
}

// NewBridge builds a bridge. hub may be nil (worker: Redis-only). rdb may be nil if channel empty.
func NewBridge(hub *ws.Hub, rdb *redis.Client, channel string) *Bridge {
	return &Bridge{hub: hub, rdb: rdb, channel: channel}
}

// Publish sends a WebSocket text payload to topic subscribers.
func (b *Bridge) Publish(ctx context.Context, topic string, body []byte) error {
	if b == nil || topic == "" || len(body) == 0 {
		return nil
	}
	if b.channel != "" && b.rdb != nil {
		c, sp := tracing.Start(ctx, "redis.ws.publish",
			attribute.String("db.system", "redis"),
			attribute.String("messaging.destination.name", b.channel),
			attribute.String("ws.topic", topic),
		)
		env, err := json.Marshal(redisEnvelope{Topic: topic, Body: body})
		if err != nil {
			tracing.Finish(sp, err)
			return err
		}
		err = b.rdb.Publish(c, b.channel, env).Err()
		tracing.Finish(sp, err)
		if err != nil {
			slog.WarnContext(ctx, "ws_event_publish_redis_failed", "topic", topic, "channel", b.channel, "error", err)
		}
		return err
	}
	if b.hub != nil {
		n := b.hub.Publish(topic, body)
		slog.DebugContext(ctx, "ws_event_publish_local", "topic", topic, "delivered", n)
	}
	return nil
}

// RunSubscriber receives Redis messages and fans out to the local Hub until ctx is cancelled.
func (b *Bridge) RunSubscriber(ctx context.Context) {
	if b == nil || b.channel == "" || b.rdb == nil || b.hub == nil {
		return
	}
	slog.InfoContext(ctx, "ws_event_subscriber_started", "channel", b.channel)
	pubsub := b.rdb.Subscribe(ctx, b.channel)
	defer func() {
		_ = pubsub.Close()
		slog.InfoContext(ctx, "ws_event_subscriber_stopped", "channel", b.channel)
	}()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg == nil {
				continue
			}
			var env redisEnvelope
			if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
				slog.WarnContext(ctx, "ws_redis_envelope_unmarshal_failed", "channel", b.channel, "error", err)
				continue
			}
			if env.Topic == "" || len(env.Body) == 0 {
				slog.DebugContext(ctx, "ws_redis_envelope_invalid", "channel", b.channel)
				continue
			}
			n := b.hub.Publish(env.Topic, []byte(env.Body))
			slog.DebugContext(ctx, "ws_redis_envelope_fanout", "channel", b.channel, "topic", env.Topic, "delivered", n)
		}
	}
}
