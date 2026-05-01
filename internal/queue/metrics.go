package queue

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector tracks queue performance metrics
type MetricsCollector struct {
	mu               sync.RWMutex
	totalEnqueued     int64
	totalDequeued     int64
	totalSuccess      int64
	totalFailed       int64
	currentProcessing int64

	// Per-job-type tracking
	jobCounts    map[JobName]int64
	jobErrors    map[JobName]int64

	// Timing
	processTimes  []time.Duration // sliding window
	processTimeMu sync.Mutex

	// Worker pool status
	workerCount     int32
	idleWorkerCount int32
	lastError       string
	lastErrorAt     time.Time
}

// NewMetricsCollector creates a metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		jobCounts: make(map[JobName]int64),
		jobErrors: make(map[JobName]int64),
	}
}

// RecordEnqueue records a job being enqueued
func (m *MetricsCollector) RecordEnqueue(name JobName, t time.Time) {
	atomic.AddInt64(&m.totalEnqueued, 1)
	m.mu.Lock()
	m.jobCounts[name]++
	m.mu.Unlock()
}

// RecordDequeue records a job being dequeued
func (m *MetricsCollector) RecordDequeue(name JobName, t time.Time) {
	atomic.AddInt64(&m.totalDequeued, 1)
}

// RecordProcessing records a job starting processing
func (m *MetricsCollector) RecordProcessing() {
	atomic.AddInt64(&m.currentProcessing, 1)
}

// RecordSuccess records a job completing successfully
func (m *MetricsCollector) RecordSuccess(jobID uint, t time.Time) {
	atomic.AddInt64(&m.totalSuccess, 1)
	atomic.AddInt64(&m.currentProcessing, -1)
}

// RecordFailure records a job failing
func (m *MetricsCollector) RecordFailure(jobID uint, errMsg string, t time.Time) {
	atomic.AddInt64(&m.totalFailed, 1)
	atomic.AddInt64(&m.currentProcessing, -1)
	m.mu.Lock()
	m.lastError = errMsg
	m.lastErrorAt = t
	m.mu.Unlock()
}

// RecordProcessTime records how long a job took to process
func (m *MetricsCollector) RecordProcessTime(d time.Duration) {
	m.processTimeMu.Lock()
	defer m.processTimeMu.Unlock()
	m.processTimes = append(m.processTimes, d)
	// Keep only last 1000 samples
	if len(m.processTimes) > 1000 {
		m.processTimes = m.processTimes[len(m.processTimes)-1000:]
	}
}

// SetWorkerCount updates the current worker pool size
func (m *MetricsCollector) SetWorkerCount(count int) {
	atomic.StoreInt32(&m.workerCount, int32(count))
}

// SetIdleWorkerCount updates the idle worker count
func (m *MetricsCollector) SetIdleWorkerCount(count int) {
	atomic.StoreInt32(&m.idleWorkerCount, int32(count))
}

// WorkerPoolStats holds worker pool status
type WorkerPoolStats struct {
	WorkerCount     int     `json:"worker_count"`
	IdleWorkerCount int     `json:"idle_worker_count"`
	AvgProcessTimeMs int64  `json:"avg_process_time_ms"`
}

// GetWorkerStats returns worker pool statistics
func (m *MetricsCollector) GetWorkerStats() WorkerPoolStats {
	stats := WorkerPoolStats{
		WorkerCount:     int(atomic.LoadInt32(&m.workerCount)),
		IdleWorkerCount: int(atomic.LoadInt32(&m.idleWorkerCount)),
	}

	m.processTimeMu.Lock()
	if len(m.processTimes) > 0 {
		var total time.Duration
		for _, d := range m.processTimes {
			total += d
		}
		stats.AvgProcessTimeMs = int64(total / time.Duration(len(m.processTimes)) / time.Millisecond)
	}
	m.processTimeMu.Unlock()

	return stats
}

// Snapshot returns a summary of all metrics
func (m *MetricsCollector) Snapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := map[string]interface{}{
		"total_enqueued":      atomic.LoadInt64(&m.totalEnqueued),
		"total_dequeued":      atomic.LoadInt64(&m.totalDequeued),
		"total_success":       atomic.LoadInt64(&m.totalSuccess),
		"total_failed":        atomic.LoadInt64(&m.totalFailed),
		"current_processing":  atomic.LoadInt64(&m.currentProcessing),
		"last_error":          m.lastError,
		"last_error_at":       m.lastErrorAt,
	}

	jobDetails := make(map[string]int64)
	for name, count := range m.jobCounts {
		jobDetails[string(name)] = count
	}
	snapshot["job_counts"] = jobDetails

	errorDetails := make(map[string]int64)
	for name, count := range m.jobErrors {
		errorDetails[string(name)] = count
	}
	snapshot["job_errors"] = errorDetails

	workerStats := m.GetWorkerStats()
	snapshot["worker_count"] = workerStats.WorkerCount
	snapshot["idle_workers"] = workerStats.IdleWorkerCount
	snapshot["avg_process_time_ms"] = workerStats.AvgProcessTimeMs

	return snapshot
}
