package worker

import (
	"context"
	"fmt"
	"sync"
)

// Pool represents a pool of workers
type Pool struct {
	maxWorkers int
	jobQueue   chan Job
	workerPool chan chan Job
	workers    []*Worker
	mu         sync.RWMutex
}

// NewPool creates a new worker pool
func NewPool(maxWorkers int) *Pool {
	pool := &Pool{
		maxWorkers: maxWorkers,
		jobQueue:   make(chan Job),
		workerPool: make(chan chan Job, maxWorkers),
		workers:    make([]*Worker, 0),
	}

	return pool
}

// Start initializes and starts the worker pool
func (p *Pool) Start(ctx context.Context) {
	// Start workers
	for i := 0; i < p.maxWorkers; i++ {
		worker := NewWorker(p.workerPool)
		p.workers = append(p.workers, worker)
		go worker.Start(ctx)
	}

	// Start job dispatcher
	go p.dispatch(ctx)
}

// Stop halts all workers in the pool
func (p *Pool) Stop() {
	for _, worker := range p.workers {
		worker.Stop()
	}
}

// Submit adds a job to the pool
func (p *Pool) Submit(job Job) {
	select {
	case p.jobQueue <- job:
	default:
		fmt.Printf("Job queue is full. Job dropped.")
	}
}

// dispatch routes jobs to available workers
func (p *Pool) dispatch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-p.jobQueue:
			// Get the next available worker job queue
			jobQueue := <-p.workerPool

			// Dispatch the job to the worker job queue
			select {
			case jobQueue <- job:
			default:
				fmt.Printf("Worker pool at capacity. Job dropped.")
			}
		}
	}
}
