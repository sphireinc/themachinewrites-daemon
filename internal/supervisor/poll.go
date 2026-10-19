package supervisor

import (
	"context"
	"sort"
	"time"
)

type Clock interface{ Now() time.Time }

type SourceSchedule struct {
	ID       string
	Interval time.Duration
	Enabled  bool
	Run      func(context.Context) error
}

type PollScheduler struct {
	clock   Clock
	minimum time.Duration
	sources []SourceSchedule
	last    map[string]time.Time
}

func NewPollScheduler(clock Clock, minimum time.Duration) *PollScheduler {
	return &PollScheduler{clock: clock, minimum: minimum, last: make(map[string]time.Time)}
}

func (s *PollScheduler) Add(source SourceSchedule) {
	s.sources = append(s.sources, source)
	sort.Slice(s.sources, func(i, j int) bool { return s.sources[i].ID < s.sources[j].ID })
}

func (s *PollScheduler) Tick(ctx context.Context) []error {
	now := s.clock.Now()
	var problems []error
	for _, source := range s.sources {
		if !source.Enabled {
			continue
		}
		interval := source.Interval
		if interval < s.minimum {
			interval = s.minimum
		}
		if last, ok := s.last[source.ID]; ok && now.Sub(last) < interval {
			continue
		}
		s.last[source.ID] = now
		if err := source.Run(ctx); err != nil {
			problems = append(problems, err)
		}
	}
	return problems
}
