package supervisor

import "time"

type Backoff struct {
	attempt   int
	base, max time.Duration
	jitter    func(int64) int64
}

func NewBackoff(base, max time.Duration, jitter func(int64) int64) *Backoff {
	return &Backoff{base: base, max: max, jitter: jitter}
}
func (b *Backoff) Next() time.Duration {
	b.attempt++
	value := b.base
	for i := 1; i < b.attempt && value < b.max; i++ {
		value *= 2
	}
	if value > b.max {
		value = b.max
	}
	if b.jitter != nil {
		factor := b.jitter(int64(value))
		value += time.Duration(factor)
	}
	if value < time.Millisecond {
		value = time.Millisecond
	}
	if value > b.max {
		value = b.max
	}
	return value
}
func (b *Backoff) Reset() { b.attempt = 0 }
