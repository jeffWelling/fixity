package checksum

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// Job represents a checksum computation job
type Job struct {
	Path      string
	Algorithm Algorithm
	Opener    func() (io.ReadCloser, error)
	Timeout   time.Duration
}

// Result represents the result of a checksum computation
type Result struct {
	Path     string
	Checksum string
	Duration time.Duration
	Error    error
}

// WorkerPool manages parallel checksum computation
type WorkerPool struct {
	workers int
	jobs    chan *Job
	results chan *Result
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(workers int) *WorkerPool {
	if workers <= 0 {
		workers = 1
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		workers: workers,
		jobs:    make(chan *Job, workers*2), // Buffer to prevent blocking
		results: make(chan *Result, workers*2),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts the worker pool
func (p *WorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// worker processes jobs from the jobs channel
func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			p.processJob(job)
		}
	}
}

// processJob computes checksum for a single job
func (p *WorkerPool) processJob(job *Job) {
	start := time.Now()

	result := &Result{
		Path: job.Path,
	}

	// Create context with timeout if specified
	ctx := p.ctx
	if job.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(p.ctx, job.Timeout)
		defer cancel()
	}

	// Open the file
	reader, err := job.Opener()
	if err != nil {
		result.Error = fmt.Errorf("failed to open file: %w", err)
		result.Duration = time.Since(start)
		p.sendResult(result)
		return
	}
	defer reader.Close()

	// Create a reader that respects context cancellation
	ctxReader := &contextReader{
		ctx:    ctx,
		reader: reader,
	}

	// Compute checksum
	checksum, err := Compute(job.Algorithm, ctxReader)
	if err != nil {
		if ctx.Err() != nil {
			result.Error = fmt.Errorf("timeout or cancelled: %w", ctx.Err())
		} else {
			result.Error = err
		}
	} else {
		result.Checksum = checksum
	}

	result.Duration = time.Since(start)
	p.sendResult(result)
}

// sendResult sends a result to the results channel
func (p *WorkerPool) sendResult(result *Result) {
	select {
	case <-p.ctx.Done():
		return
	case p.results <- result:
	}
}

// Submit submits a job to the worker pool
func (p *WorkerPool) Submit(job *Job) error {
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("worker pool is stopped")
	case p.jobs <- job:
		return nil
	}
}

// Results returns the results channel
func (p *WorkerPool) Results() <-chan *Result {
	return p.results
}

// Stop stops the worker pool and waits for all workers to finish
func (p *WorkerPool) Stop() {
	close(p.jobs)
	p.wg.Wait()
	close(p.results)
	p.cancel()
}

// contextReader wraps an io.Reader to respect context cancellation
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

// Read implements io.Reader with context cancellation support
func (r *contextReader) Read(p []byte) (int, error) {
	// Check if context is cancelled
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
	}

	// Perform the read
	return r.reader.Read(p)
}
