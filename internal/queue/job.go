package queue

import (
	"context"
	"time"
)

// JobStatus represents the status of a job in the queue
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusRetrying   JobStatus = "retrying"
	JobStatusDead       JobStatus = "dead" // dead letter queue
)

// JobName identifies a job type uniquely
type JobName string

const (
	JobOrderHandle    JobName = "order_handle"
	JobSendMail       JobName = "send_mail"
	JobSendTelegram   JobName = "send_telegram"
	JobTrafficFetch   JobName = "traffic_fetch"
	JobStatUser       JobName = "stat_user"
	JobStatServer     JobName = "stat_server"
	JobLogAudit       JobName = "log_audit"
)

// JobPayload is the serializable data for a job
type JobPayload map[string]interface{}

// Job represents a unit of work in the queue system
type Job struct {
	ID          uint       `json:"id"`
	Name        JobName    `json:"name"`
	Payload     JobPayload `json:"payload"`
	Status      JobStatus  `json:"status"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	Priority    int        `json:"priority"`  // higher = more urgent
	ErrorMsg    string     `json:"error_msg,omitempty"`
	QueueName   string     `json:"queue_name"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"` // for delayed jobs
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
}

// JobHandler processes a job and returns an error if processing failed
type JobHandler func(ctx context.Context, job *Job) error

// Queue is the core interface for queue operations
type Queue interface {
	// Push adds a job to the queue
	Push(ctx context.Context, job *Job) error

	// Pop retrieves and removes the next job from the queue (blocking)
	Pop(ctx context.Context, queueName string) (*Job, error)

	// Acknowledge marks a job as successfully processed
	Acknowledge(ctx context.Context, jobID uint) error

	// Fail marks a job as failed and handles retry/dead-letter logic
	Fail(ctx context.Context, jobID uint, errMsg string) error

	// Release puts a job back in queue for retry
	Release(ctx context.Context, jobID uint, delay time.Duration) error

	// Recover requeues jobs that are stuck in "processing" state
	Recover(ctx context.Context) (int, error)

	// Stats returns queue statistics
	Stats(ctx context.Context) (*QueueStats, error)

	// Close cleans up resources
	Close() error
}

// QueueStats holds queue monitoring metrics
type QueueStats struct {
	PendingCount     int            `json:"pending_count"`
	ProcessingCount  int            `json:"processing_count"`
	CompletedCount   int64          `json:"completed_count"`
	FailedCount      int64          `json:"failed_count"`
	DeadCount        int            `json:"dead_count"`
	WorkerCount      int            `json:"worker_count"`
	IdleWorkerCount  int            `json:"idle_worker_count"`
	AvgProcessTimeMs int64          `json:"avg_process_time_ms"`
	QueueDepths      map[string]int `json:"queue_depths"`
	ErrorRate        float64        `json:"error_rate"`
}

// NewJob creates a new job with sensible defaults
func NewJob(name JobName, payload JobPayload, opts ...JobOption) *Job {
	now := time.Now()
	job := &Job{
		Name:        name,
		Payload:     payload,
		Status:      JobStatusPending,
		Attempts:    0,
		MaxAttempts: 3,
		Priority:    0,
		QueueName:   "default",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	for _, opt := range opts {
		opt(job)
	}
	return job
}

// JobOption allows configuring a job
type JobOption func(*Job)

// WithMaxAttempts sets the max retry attempts
func WithMaxAttempts(n int) JobOption {
	return func(j *Job) { j.MaxAttempts = n }
}

// WithPriority sets job priority
func WithPriority(p int) JobOption {
	return func(j *Job) { j.Priority = p }
}

// WithQueueName sets the target queue name
func WithQueueName(name string) JobOption {
	return func(j *Job) { j.QueueName = name }
}

// WithScheduledAt sets the scheduled time (for delayed execution)
func WithScheduledAt(t time.Time) JobOption {
	return func(j *Job) { j.ScheduledAt = &t }
}
