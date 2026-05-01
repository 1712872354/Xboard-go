package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// redisQueuePrefix is the Redis key prefix for queue lists
const redisQueuePrefix = "queue:"

// Engine implements the Queue interface using Redis for fast queuing
// and MySQL for persistent storage and retry management.
//
// Architecture:
//   - Producer:  Job → MySQL (persist) → Redis LPUSH (fast notify)
//   - Consumer:  Redis BRPOP (blocking pop) → MySQL (processing) → Execute → ACK/FAIL
//   - Recovery:  Stuck "processing" jobs recovered on startup via cron
type Engine struct {
	db        *gorm.DB
	rdb       *redis.Client
	mu        sync.RWMutex
	stopped   bool
	stopCh    chan struct{}
	metrics   *MetricsCollector
}

// NewEngine creates a new queue engine
func NewEngine(db *gorm.DB, rdb *redis.Client) *Engine {
	return &Engine{
		db:      db,
		rdb:     rdb,
		stopCh:  make(chan struct{}),
		metrics: NewMetricsCollector(),
	}
}

// redisListKey returns the Redis key for a given queue
func redisListKey(queueName string) string {
	return redisQueuePrefix + queueName
}

// Push persists a job to MySQL and pushes its ID to Redis for fast consumption
func (e *Engine) Push(ctx context.Context, job *Job) error {
	// 1. Create persistent record in MySQL
	record := FromJob(job)
	if err := e.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("queue: persist job failed: %w", err)
	}

	// 2. Push job ID + priority to Redis sorted set for ordering,
	//    and to a list for fast dequeue
	job.ID = record.ID
	job.CreatedAt = record.CreatedAt
	job.UpdatedAt = record.UpdatedAt

	pipe := e.rdb.Pipeline()

	// LPUSH for O(1) dequeue
	pipe.LPush(ctx, redisListKey(job.QueueName), job.ID)

	// ZADD for priority-sorted retrieval and monitoring
	score := float64(time.Now().UnixNano())
	if job.ScheduledAt != nil && job.ScheduledAt.After(time.Now()) {
		score = float64(job.ScheduledAt.UnixNano())
	}
	pipe.ZAdd(ctx, redisListKey(job.QueueName)+":zset", redis.Z{
		Score:  score,
		Member: job.ID,
	})

	_, err := pipe.Exec(ctx)
	if err != nil {
		// Redis failed - job is still in MySQL (safe)
		log.Printf("queue: redis push failed (job %d persisted): %v", job.ID, err)
	}

	e.metrics.RecordEnqueue(job.Name, time.Now())
	return nil
}

// Pop retrieves the next job from Redis queue. Blocks until a job is available.
func (e *Engine) Pop(ctx context.Context, queueName string) (*Job, error) {
	listKey := redisListKey(queueName)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// BRPOP with 5-second timeout for periodic checks
		result, err := e.rdb.BRPop(ctx, 5*time.Second, listKey).Result()
		if err == redis.Nil {
			// Timeout, check for scheduled/delayed jobs
			if err := e.flushScheduledJobs(ctx, queueName); err != nil {
				log.Printf("queue: flush scheduled jobs error: %v", err)
			}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("queue: pop failed: %w", err)
		}

		// result[0] = key, result[1] = value (job ID)
		if len(result) < 2 {
			continue
		}

		var jobID uint
		if _, err := fmt.Sscanf(result[1], "%d", &jobID); err != nil {
			continue
		}

		// Load full job details from MySQL
		job, err := e.loadJob(ctx, jobID)
		if err != nil {
			log.Printf("queue: load job %d failed: %v", jobID, err)
			continue
		}

		// Double check: skip if scheduled for later
		if job.ScheduledAt != nil && job.ScheduledAt.After(time.Now()) {
			// Re-push to Redis with a delay
			delay := job.ScheduledAt.Sub(time.Now())
			time.AfterFunc(delay, func() {
				e.rdb.LPush(context.Background(), listKey, jobID)
			})
			continue
		}

		// Skip if dead or already completed
		if job.Status == JobStatusDead || job.Status == JobStatusCompleted {
			continue
		}

		// Mark as processing in MySQL
		now := time.Now()
		e.db.WithContext(ctx).Model(&QueueJobRecord{}).
			Where("id = ?", jobID).
			Updates(map[string]interface{}{
				"status":       string(JobStatusProcessing),
				"attempts":     gorm.Expr("attempts + 1"),
				"updated_at":   now,
			})

		job.Status = JobStatusProcessing
		job.Attempts++
		job.UpdatedAt = now

		e.metrics.RecordDequeue(job.Name, time.Now())
		return job, nil
	}
}

// Acknowledge marks a job as completed
func (e *Engine) Acknowledge(ctx context.Context, jobID uint) error {
	now := time.Now()
	result := e.db.WithContext(ctx).Model(&QueueJobRecord{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":       string(JobStatusCompleted),
			"processed_at": &now,
			"updated_at":   now,
		})
	if result.Error != nil {
		return fmt.Errorf("queue: acknowledge job %d failed: %w", jobID, result.Error)
	}

	e.metrics.RecordSuccess(jobID, time.Now())
	return nil
}

// Fail handles a job failure with retry logic
func (e *Engine) Fail(ctx context.Context, jobID uint, errMsg string) error {
	record := &QueueJobRecord{}
	if err := e.db.WithContext(ctx).First(record, jobID).Error; err != nil {
		return err
	}

	now := time.Now()
	updates := map[string]interface{}{
		"error_msg":  errMsg,
		"updated_at": now,
	}

	if record.Attempts >= record.MaxAttempts {
		// Move to dead letter queue after exhausting retries
		updates["status"] = string(JobStatusDead)
		updates["processed_at"] = &now
		log.Printf("queue: job %d (%s) moved to DLQ after %d attempts: %s",
			jobID, record.Name, record.Attempts, errMsg)
	} else {
		// Schedule retry with exponential backoff
		delay := exponentialBackoff(record.Attempts)
		nextRetry := now.Add(delay)
		updates["status"] = string(JobStatusRetrying)
		updates["next_retry_at"] = &nextRetry

		log.Printf("queue: job %d (%s) failed (attempt %d/%d), retrying in %v: %s",
			jobID, record.Name, record.Attempts, record.MaxAttempts, delay, errMsg)
	}

	if err := e.db.WithContext(ctx).Model(&QueueJobRecord{}).
		Where("id = ?", jobID).Updates(updates).Error; err != nil {
		return err
	}

	e.metrics.RecordFailure(jobID, errMsg, time.Now())
	return nil
}

// Release puts a job back in the queue for retry (after the retry delay)
func (e *Engine) Release(ctx context.Context, jobID uint, delay time.Duration) error {
	// Update the next_retry_at field
	nextRetry := time.Now().Add(delay)
	if err := e.db.WithContext(ctx).Model(&QueueJobRecord{}).
		Where("id = ?", jobID).
		Updates(map[string]interface{}{
			"status":       string(JobStatusPending),
			"next_retry_at": &nextRetry,
		}).Error; err != nil {
		return err
	}

	// Schedule re-push to Redis after delay
	time.AfterFunc(delay, func() {
		e.rdb.LPush(context.Background(), redisListKey("default"), jobID)
	})

	return nil
}

// Recover finds all jobs stuck in "processing" state and requeues them.
// Called on startup and periodically via cron.
func (e *Engine) Recover(ctx context.Context) (int, error) {
	timeout := time.Now().Add(-10 * time.Minute) // jobs processing >10min are stuck
	var stuckJobs []QueueJobRecord

	if err := e.db.WithContext(ctx).
		Model(&QueueJobRecord{}).
		Where("status = ? AND updated_at < ?", JobStatusProcessing, timeout).
		Find(&stuckJobs).Error; err != nil {
		return 0, err
	}

	count := 0
	for _, job := range stuckJobs {
		// Reset to pending so it can be picked up again
		e.db.Model(&job).Updates(map[string]interface{}{
			"status":     string(JobStatusPending),
			"updated_at": time.Now(),
		})
		// Re-push to Redis
		e.rdb.LPush(ctx, redisListKey(job.QueueName), job.ID)
		count++
	}

	if count > 0 {
		log.Printf("queue: recovered %d stuck jobs", count)
	}
	return count, nil
}

// Stats returns queue statistics
func (e *Engine) Stats(ctx context.Context) (*QueueStats, error) {
	stats := &QueueStats{
		QueueDepths: make(map[string]int),
	}

	// Count by status
	var pendingCount, processingCount, deadCount int64
	e.db.WithContext(ctx).Model(&QueueJobRecord{}).Where("status = ?", JobStatusPending).Count(&pendingCount)
	e.db.WithContext(ctx).Model(&QueueJobRecord{}).Where("status = ?", JobStatusProcessing).Count(&processingCount)
	e.db.WithContext(ctx).Model(&QueueJobRecord{}).Where("status = ?", JobStatusDead).Count(&deadCount)
	stats.PendingCount = int(pendingCount)
	stats.ProcessingCount = int(processingCount)
	stats.DeadCount = int(deadCount)

	// Total completed/failed
	e.db.WithContext(ctx).Model(&QueueJobRecord{}).Where("status = ?", JobStatusCompleted).Count(&stats.CompletedCount)
	e.db.WithContext(ctx).Model(&QueueJobRecord{}).Where("status = ?", JobStatusFailed).Count(&stats.FailedCount)

	// Error rate
	total := stats.CompletedCount + stats.FailedCount
	if total > 0 {
		stats.ErrorRate = float64(stats.FailedCount) / float64(total) * 100
	}

	// Worker stats from metrics
	workerStats := e.metrics.GetWorkerStats()
	stats.WorkerCount = workerStats.WorkerCount
	stats.IdleWorkerCount = workerStats.IdleWorkerCount
	stats.AvgProcessTimeMs = workerStats.AvgProcessTimeMs

	// Redis list lengths for each active queue
	queues := []string{"default", "high", "low", "mail", "telegram", "stat", "order"}
	for _, q := range queues {
		len, err := e.rdb.LLen(ctx, redisListKey(q)).Result()
		if err == nil && len > 0 {
			stats.QueueDepths[q] = int(len)
		}
	}

	return stats, nil
}

// Close gracefully shuts down the engine
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopped = true
	close(e.stopCh)
	return nil
}

// flushScheduledJobs moves scheduled/delayed jobs from MySQL to Redis
func (e *Engine) flushScheduledJobs(ctx context.Context, queueName string) error {
	now := time.Now()
	var readyJobs []QueueJobRecord

	if err := e.db.WithContext(ctx).
		Model(&QueueJobRecord{}).
		Where("status = ? AND queue_name = ? AND scheduled_at IS NOT NULL AND scheduled_at <= ?",
			JobStatusPending, queueName, now).
		Limit(100).
		Find(&readyJobs).Error; err != nil {
		return err
	}

	for _, job := range readyJobs {
		e.rdb.LPush(ctx, redisListKey(queueName), job.ID)
	}

	return nil
}

// loadJob loads a complete job from MySQL
func (e *Engine) loadJob(ctx context.Context, jobID uint) (*Job, error) {
	var record QueueJobRecord
	if err := e.db.WithContext(ctx).First(&record, jobID).Error; err != nil {
		return nil, err
	}
	return record.ToJob(), nil
}

// exponentialBackoff calculates retry delay with jitter
// Attempt 0: 5s, 1: 10s, 2: 20s, 3: 40s, 4: 80s, ...
func exponentialBackoff(attempt int) time.Duration {
	baseSeconds := float64(5)
	maxSeconds := float64(3600) // max 1 hour
	delay := math.Min(baseSeconds*math.Pow(2, float64(attempt)), maxSeconds)
	return time.Duration(delay) * time.Second
}

// MustJSON serializes a JobPayload to JSON string (panics on error - for internal use)
func MustJSON(payload JobPayload) string {
	data, err := json.Marshal(payload)
	if err != nil {
		panic("queue: failed to marshal payload: " + err.Error())
	}
	return string(data)
}

// Metrics returns the metrics collector for monitoring
func (e *Engine) Metrics() *MetricsCollector {
	return e.metrics
}

