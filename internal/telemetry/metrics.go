package telemetry

import "sync"

const histogramBuckets = 6

type Metrics struct {
	mu             sync.Mutex
	invocations    uint64
	sourceErrors   uint64
	publications   uint64
	retries        uint64
	durationCounts [histogramBuckets]uint64
}

type Snapshot struct {
	Invocations       uint64   `json:"invocations"`
	SourceErrors      uint64   `json:"source_errors"`
	Publications      uint64   `json:"publications"`
	Retries           uint64   `json:"retries"`
	DurationHistogram []uint64 `json:"duration_histogram"`
}

func (m *Metrics) ModelInvoked()  { m.mu.Lock(); m.invocations++; m.mu.Unlock() }
func (m *Metrics) SourceErrored() { m.mu.Lock(); m.sourceErrors++; m.mu.Unlock() }
func (m *Metrics) Published()     { m.mu.Lock(); m.publications++; m.mu.Unlock() }
func (m *Metrics) Retried()       { m.mu.Lock(); m.retries++; m.mu.Unlock() }

func (m *Metrics) ObserveDurationMilliseconds(value uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	bucket := 0
	for bucket+1 < histogramBuckets && value >= uint64(100)<<bucket {
		bucket++
	}
	m.durationCounts[bucket]++
}

func (m *Metrics) Reset() {
	m.mu.Lock()
	m.invocations = 0
	m.sourceErrors = 0
	m.publications = 0
	m.retries = 0
	m.durationCounts = [histogramBuckets]uint64{}
	m.mu.Unlock()
}

func (m *Metrics) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Snapshot{
		Invocations: m.invocations, SourceErrors: m.sourceErrors, Publications: m.publications,
		Retries: m.retries, DurationHistogram: append([]uint64(nil), m.durationCounts[:]...),
	}
}
