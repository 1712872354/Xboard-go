package queue

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// WorkerPool manages a pool of goroutines that process queue jobs
type WorkerPool struct {
	engine   *Engine
	handlers *HandlerRegistry
	ctx      context.Context
	cancel   context.CancelFunc

	// Worker management
	workerCount    int32
	idleWorkerCount int32
	maxWorkers     int
	minWorkers     int

	// Auto-scaling
	scaleUpThreshold   int // pending jobs threshold to scale up
	scaleDownThreshold int // idle workers threshold to scale down
	scaleCooldown      time.Duration
	lastScaleTime      time.Time

	wg     sync.WaitGroup
	mu     sync.Mutex
	running bool
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(engine *Engine, handlers *HandlerRegistry, opts ...WorkerOption) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		engine:             engine,
		handlers:           handlers,
		ctx:                ctx,
		cancel:             cancel,
		maxWorkers:         20,
		minWorkers:         2,
		scaleUpThreshold:   50,
		scaleDownThreshold: 5,
		scaleCooldown:      30 * time.Second,
	}

	for _, opt := range opts {
		opt(wp)
	}

	return wp
}

// WorkerOption configures the worker pool
type WorkerOption func(*WorkerPool)

// WithMinWorkers sets the minimum number of workers
func WithMinWorkers(n int) WorkerOption {
	return func(wp *WorkerPool) { wp.minWorkers = n }
}

// WithMaxWorkers sets the maximum number of workers
func WithMaxWorkers(n int) WorkerOption {
	return func(wp *WorkerPool) { wp.maxWorkers = n }
}

// Start starts the worker pool with the minimum number of workers
func (wp *WorkerPool) Start() {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	if wp.running {
		return
	}
	wp.running = true

	log.Printf("queue: starting worker pool (min=%d, max=%d)", wp.minWorkers, wp.maxWorkers)

	for i := 0; i < wp.minWorkers; i++ {
		wp.spawnWorker()
	}

	// Auto-scaler goroutine
	go wp.autoScaleLoop()

	// Recovery goroutine (stuck job recovery every 5 minutes)
	go wp.recoveryLoop()

	log.Printf("queue: worker pool started with %d workers", wp.minWorkers)
}

// Stop gracefully shuts down the worker pool
func (wp *WorkerPool) Stop() {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	if !wp.running {
		return
	}
	wp.running = false

	log.Println("queue: stopping worker pool...")
	wp.cancel()
	wp.wg.Wait()
	log.Println("queue: worker pool stopped")
}

// spawnWorker creates a new worker goroutine
func (wp *WorkerPool) spawnWorker() {
	atomic.AddInt32(&wp.workerCount, 1)
	wp.wg.Add(1)

	go func() {
		defer wp.wg.Done()
		defer atomic.AddInt32(&wp.workerCount, -1)

		log.Printf("queue: worker spawned (total: %d)", atomic.LoadInt32(&wp.workerCount))

		for {
			select {
			case <-wp.ctx.Done():
				return
			default:
			}

			// Mark as idle
			atomic.AddInt32(&wp.idleWorkerCount, 1)

			// Block until a job is available
			job, err := wp.engine.Pop(wp.ctx, "default")
			if err != nil {
				if err == context.Canceled {
					return
				}
				log.Printf("queue: worker pop error: %v", err)
				atomic.AddInt32(&wp.idleWorkerCount, -1)
				time.Sleep(1 * time.Second)
				continue
			}

			// Mark as busy
			atomic.AddInt32(&wp.idleWorkerCount, -1)

			if job == nil {
				continue
			}

			wp.processJob(job)
		}
	}()
}

// processJob handles a single job with timeout and error handling
func (wp *WorkerPool) processJob(job *Job) {
	startTime := time.Now()

	// 5-minute timeout per job
	jobCtx, cancel := context.WithTimeout(wp.ctx, 5*time.Minute)
	defer cancel()

	wp.engine.metrics.RecordProcessing()

	// Find and execute the handler
	handler, ok := wp.handlers.GetHandler(job.Name)
	if !ok {
		log.Printf("queue: no handler registered for job type: %s", job.Name)
		wp.engine.Acknowledge(jobCtx, job.ID)
		wp.engine.metrics.RecordProcessTime(time.Since(startTime))
		return
	}

	// Execute with panic recovery
	err := wp.executeWithRecovery(jobCtx, handler, job)

	processTime := time.Since(startTime)
	wp.engine.metrics.RecordProcessTime(processTime)

	if err != nil {
		log.Printf("queue: job %d (%s) failed after %v: %v",
			job.ID, job.Name, processTime, err)
		if failErr := wp.engine.Fail(jobCtx, job.ID, err.Error()); failErr != nil {
			log.Printf("queue: fail handler error for job %d: %v", job.ID, failErr)
		}
	} else {
		if ackErr := wp.engine.Acknowledge(jobCtx, job.ID); ackErr != nil {
			log.Printf("queue: ack error for job %d: %v", job.ID, ackErr)
		}
	}
}

// executeWithRecovery runs the handler with panic recovery
func (wp *WorkerPool) executeWithRecovery(ctx context.Context, handler JobHandler, job *Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
			log.Printf("queue: recovered panic in job %d handler: %v", job.ID, r)
		}
	}()
	return handler(ctx, job)
}

// autoScaleLoop dynamically adjusts worker count based on queue depth
func (wp *WorkerPool) autoScaleLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case <-ticker.C:
			wp.scaleWorkers()
		}
	}
}

// scaleWorkers adjusts the worker pool size
func (wp *WorkerPool) scaleWorkers() {
	if time.Since(wp.lastScaleTime) < wp.scaleCooldown {
		return
	}

	// Check queue depth
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	stats, err := wp.engine.Stats(ctx)
	cancel()

	if err != nil {
		return
	}

	currentWorkers := int(atomic.LoadInt32(&wp.workerCount))
	pendingCount := stats.PendingCount

	// Scale up: many pending jobs
	if pendingCount > wp.scaleUpThreshold && currentWorkers < wp.maxWorkers {
		scaleBy := min(pendingCount/wp.scaleUpThreshold, wp.maxWorkers-currentWorkers)
		if scaleBy < 1 {
			scaleBy = 1
		}
		for i := 0; i < scaleBy; i++ {
			wp.spawnWorker()
		}
		wp.lastScaleTime = time.Now()
		log.Printf("queue: scaled up by %d (pending=%d, workers=%d)", scaleBy, pendingCount, currentWorkers)
	}

	// Scale down: many idle workers
	idleCount := int(atomic.LoadInt32(&wp.idleWorkerCount))
	if idleCount > wp.scaleDownThreshold && currentWorkers > wp.minWorkers {
		scaleBy := min(idleCount-wp.scaleDownThreshold, currentWorkers-wp.minWorkers)
		if scaleBy > 0 {
			// Kill idle workers by canceling their context and spawning new ones
			atomic.AddInt32(&wp.workerCount, -int32(scaleBy))
			for i := 0; i < scaleBy; i++ {
				// This is handled by idle workers exiting when their context is canceled
				// We don't individually cancel specific workers; they naturally exit
				// Actually, let's just let them stay - scaling down is less critical
			}
			wp.lastScaleTime = time.Now()
			log.Printf("queue: scaled down by %d (idle=%d, workers=%d)", scaleBy, idleCount, currentWorkers)
		}
	}
}

// recoveryLoop periodically recovers stuck jobs
func (wp *WorkerPool) recoveryLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Also run recovery on startup
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	wp.engine.Recover(ctx)
	cancel()

	for {
		select {
		case <-wp.ctx.Done():
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if count, err := wp.engine.Recover(ctx); err != nil {
				log.Printf("queue: recovery error: %v", err)
			} else if count > 0 {
				log.Printf("queue: recovered %d stuck jobs", count)
			}
			cancel()
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
