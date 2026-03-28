package auction

import (
	"math"
	"sort"
	"sync"
	"time"
)

// BidMetrics holds experiment metrics.
type BidMetrics struct {
	TotalBids             int64   `json:"total_bids"`
	SuccessfulBids        int64   `json:"successful_bids"`
	RejectedBids          int64   `json:"rejected_bids"`
	AvgLatencyMs          float64 `json:"avg_latency_ms"`
	P95LatencyMs          float64 `json:"p95_latency_ms"`
	P99LatencyMs          float64 `json:"p99_latency_ms"`
	ConsistencyViolations int64   `json:"consistency_violations"`
}

// Metrics collects bid latency and outcome data for experiments.
type Metrics struct {
	mu        sync.Mutex
	success   int64
	rejected  int64
	latencies []float64 // in milliseconds
	violations int64
}

// NewMetrics creates a new Metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{
		latencies: make([]float64, 0, 1024),
	}
}

// RecordSuccessful records a successful bid with its latency.
func (m *Metrics) RecordSuccessful(latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.success++
	m.latencies = append(m.latencies, float64(latency.Microseconds())/1000.0)
}

// RecordRejected records a rejected bid.
func (m *Metrics) RecordRejected() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rejected++
}

// RecordViolation records a consistency violation.
func (m *Metrics) RecordViolation() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.violations++
}

// Snapshot returns a point-in-time copy of the metrics.
func (m *Metrics) Snapshot() *BidMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	total := m.success + m.rejected
	snap := &BidMetrics{
		TotalBids:             total,
		SuccessfulBids:        m.success,
		RejectedBids:          m.rejected,
		ConsistencyViolations: m.violations,
	}

	if len(m.latencies) > 0 {
		sorted := make([]float64, len(m.latencies))
		copy(sorted, m.latencies)
		sort.Float64s(sorted)

		var sum float64
		for _, l := range sorted {
			sum += l
		}
		snap.AvgLatencyMs = math.Round(sum/float64(len(sorted))*100) / 100
		snap.P95LatencyMs = percentile(sorted, 0.95)
		snap.P99LatencyMs = percentile(sorted, 0.99)
	}

	return snap
}

// Reset clears all metrics.
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.success = 0
	m.rejected = 0
	m.latencies = m.latencies[:0]
	m.violations = 0
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return math.Round(sorted[idx]*100) / 100
}
