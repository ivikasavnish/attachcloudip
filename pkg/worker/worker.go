package worker

import (
	"context"
	"fmt"
)

// Job represents a job that can be executed by a worker
type Job interface {
	Execute(ctx context.Context) error
}

// Worker represents a worker that executes jobs
type Worker struct {
	id         int
	jobQueue   chan Job
	workerPool chan chan Job
	quit       chan bool
}

// NewWorker creates a new worker
func NewWorker(workerPool chan chan Job) *Worker {
	return &Worker{
		jobQueue:   make(chan Job),
		workerPool: workerPool,
		quit:       make(chan bool),
	}
}

// Start begins the worker's processing loop
func (w *Worker) Start(ctx context.Context) {
	go func() {
		for {
			// Add the worker's job queue to the pool
			w.workerPool <- w.jobQueue

			select {
			case job := <-w.jobQueue:
				// Execute the job
				if err := job.Execute(ctx); err != nil {
					fmt.Printf("Error executing job: %v\n", err)
				}

			case <-w.quit:
				// Stop processing jobs
				return

			case <-ctx.Done():
				// Context cancelled, stop processing
				return
			}
		}
	}()
}

// Stop signals the worker to stop processing jobs
func (w *Worker) Stop() {
	w.quit <- true
}
