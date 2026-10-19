package telemetry

import (
	"context"
	"log/slog"
	"time"

	"github.com/TheMachineWrites/themachinewrites-daemon/internal/logging"
)

type Event string

const (
	EventSourcePoll    Event = "source_poll"
	EventCandidate     Event = "candidate_queued"
	EventModelStarted  Event = "model_started"
	EventModelDeferred Event = "model_deferred"
	EventPostStaged    Event = "post_staged"
	EventPushConfirmed Event = "push_confirmed"
	EventCycleFailed   Event = "cycle_failed"
)

var eventAllowlist = map[Event]bool{
	EventSourcePoll: true, EventCandidate: true, EventModelStarted: true,
	EventModelDeferred: true, EventPostStaged: true, EventPushConfirmed: true,
	EventCycleFailed: true,
}

type Emitter struct{ logger *slog.Logger }

func NewEmitter(logger *slog.Logger) *Emitter { return &Emitter{logger: logger} }

func (e *Emitter) Emit(ctx context.Context, event Event, jobID string, duration time.Duration, attrs ...any) {
	if e == nil || e.logger == nil {
		return
	}
	level := slog.LevelInfo
	if event == EventCycleFailed {
		level = slog.LevelError
	}
	normalized := eventAllowlistValue(event)
	args := []any{"event", string(normalized), "job_id", jobID}
	if duration >= 0 {
		args = append(args, "duration", duration)
	}
	args = append(args, attrs...)
	if !eventAllowlist[event] {
		args = append(args, "error", "unknown lifecycle event")
		level = slog.LevelError
	}
	logging.FromLogger(e.logger).Log(ctx, level, string(normalized), args...)
}

func eventAllowlistValue(event Event) Event {
	if eventAllowlist[event] {
		return event
	}
	return EventCycleFailed
}
