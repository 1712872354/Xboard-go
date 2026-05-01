package queue

import (
	"context"
	"log"
)

// Dispatcher provides a simple interface for services to dispatch jobs
type Dispatcher struct {
	engine *Engine
}

// NewDispatcher creates a new job dispatcher
func NewDispatcher(engine *Engine) *Dispatcher {
	return &Dispatcher{engine: engine}
}

// DispatchOrderHandle queues an order handling job
func (d *Dispatcher) DispatchOrderHandle(ctx context.Context, orderID uint, action string) {
	job := NewJob(JobOrderHandle, JobPayload{
		"order_id": float64(orderID),
		"action":   action,
	}, WithQueueName("order"), WithMaxAttempts(5))

	if err := d.engine.Push(ctx, job); err != nil {
		log.Printf("queue: dispatch order_handle failed: %v", err)
	}
}

// DispatchSendMail queues an email sending job
func (d *Dispatcher) DispatchSendMail(ctx context.Context, to, subject, body string) {
	job := NewJob(JobSendMail, JobPayload{
		"to":      to,
		"subject": subject,
		"body":    body,
	}, WithQueueName("mail"), WithMaxAttempts(3))

	if err := d.engine.Push(ctx, job); err != nil {
		log.Printf("queue: dispatch send_mail failed: %v", err)
	}
}

// DispatchSendTelegram queues a Telegram notification job
func (d *Dispatcher) DispatchSendTelegram(ctx context.Context, chatID int64, message string) {
	job := NewJob(JobSendTelegram, JobPayload{
		"chat_id": float64(chatID),
		"message": message,
	}, WithQueueName("telegram"), WithMaxAttempts(3))

	if err := d.engine.Push(ctx, job); err != nil {
		log.Printf("queue: dispatch send_telegram failed: %v", err)
	}
}

// DispatchStatUser queues a user stats recording job
func (d *Dispatcher) DispatchStatUser(ctx context.Context, userID uint) {
	job := NewJob(JobStatUser, JobPayload{
		"user_id": float64(userID),
	}, WithQueueName("stat"), WithMaxAttempts(2))

	if err := d.engine.Push(ctx, job); err != nil {
		log.Printf("queue: dispatch stat_user failed: %v", err)
	}
}

// DispatchStatServer queues a server stats recording job
func (d *Dispatcher) DispatchStatServer(ctx context.Context, serverID uint) {
	job := NewJob(JobStatServer, JobPayload{
		"server_id": float64(serverID),
	}, WithQueueName("stat"), WithMaxAttempts(2))

	if err := d.engine.Push(ctx, job); err != nil {
		log.Printf("queue: dispatch stat_server failed: %v", err)
	}
}

// DispatchTrafficFetch queues a traffic fetch job
func (d *Dispatcher) DispatchTrafficFetch(ctx context.Context, serverID uint) {
	job := NewJob(JobTrafficFetch, JobPayload{
		"server_id": float64(serverID),
	}, WithQueueName("default"), WithMaxAttempts(3))

	if err := d.engine.Push(ctx, job); err != nil {
		log.Printf("queue: dispatch traffic_fetch failed: %v", err)
	}
}

// DispatchLogAudit queues an audit log job
func (d *Dispatcher) DispatchLogAudit(ctx context.Context, userID uint, action, ip, details string) {
	job := NewJob(JobLogAudit, JobPayload{
		"user_id": float64(userID),
		"action":  action,
		"ip":      ip,
		"details": details,
	}, WithQueueName("default"), WithMaxAttempts(2))

	if err := d.engine.Push(ctx, job); err != nil {
		log.Printf("queue: dispatch log_audit failed: %v", err)
	}
}
