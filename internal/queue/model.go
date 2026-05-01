package queue

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// QueueJobRecord is the GORM model for persistent queue job storage in MySQL
type QueueJobRecord struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	Name          string     `gorm:"type:varchar(64);index:idx_name_status;not null" json:"name"`
	Payload       JSON       `gorm:"type:json" json:"payload"`
	Status        string     `gorm:"type:varchar(20);index:idx_name_status;index:idx_status;default:pending" json:"status"`
	Attempts      int        `gorm:"type:int(11);default:0" json:"attempts"`
	MaxAttempts   int        `gorm:"type:int(11);default:3" json:"max_attempts"`
	Priority      int        `gorm:"type:int(11);default:0;index" json:"priority"`
	ErrorMsg      string     `gorm:"type:text" json:"error_msg,omitempty"`
	QueueName     string     `gorm:"type:varchar(64);default:default;index" json:"queue_name"`
	Result        string     `gorm:"type:text" json:"result,omitempty"`
	ScheduledAt   *time.Time `gorm:"type:datetime;index" json:"scheduled_at,omitempty"`
	NextRetryAt   *time.Time `gorm:"type:datetime;index" json:"next_retry_at,omitempty"`
	ProcessedAt   *time.Time `gorm:"type:datetime" json:"processed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// TableName returns the table name
func (QueueJobRecord) TableName() string {
	return "queue_jobs"
}

// JSON for the queue job record
type JSON map[string]interface{}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSON: unexpected type %T", value)
	}
	return json.Unmarshal(bytes, j)
}

func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// ToJob converts a QueueJobRecord to a Job
func (r *QueueJobRecord) ToJob() *Job {
	return &Job{
		ID:          r.ID,
		Name:        JobName(r.Name),
		Payload:     JobPayload(r.Payload),
		Status:      JobStatus(r.Status),
		Attempts:    r.Attempts,
		MaxAttempts: r.MaxAttempts,
		Priority:    r.Priority,
		ErrorMsg:    r.ErrorMsg,
		QueueName:   r.QueueName,
		ScheduledAt: r.ScheduledAt,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		NextRetryAt: r.NextRetryAt,
	}
}

// FromJob creates a QueueJobRecord from a Job
func FromJob(job *Job) *QueueJobRecord {
	payload := make(JSON)
	for k, v := range job.Payload {
		payload[k] = v
	}
	return &QueueJobRecord{
		Name:        string(job.Name),
		Payload:     payload,
		Status:      string(job.Status),
		Attempts:    job.Attempts,
		MaxAttempts: job.MaxAttempts,
		Priority:    job.Priority,
		ErrorMsg:    job.ErrorMsg,
		QueueName:   job.QueueName,
		ScheduledAt: job.ScheduledAt,
		NextRetryAt: job.NextRetryAt,
	}
}
