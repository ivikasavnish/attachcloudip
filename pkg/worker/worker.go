package worker

import (
	"log"
	"sync"
)

// Worker represents an individual worker that processes jobs
type Worker struct {
	jobQueue   chan Job
	workerPool chan chan Job
	stop       chan struct{}
	wg         sync.WaitGroup
}

// NewWorker creates a new Worker
func NewWorker(workerPool chan chan Job) *Worker {
	return &Worker{
		jobQueue:   make(chan Job),
		workerPool: workerPool,
		stop:       make(chan struct{}),
	}
}

// Start begins the worker's job processing loop
func (w *Worker) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		for {
			// Register this worker's job queue to the worker pool
			w.workerPool <- w.jobQueue

			select {
			case job := <-w.jobQueue:
				// Process the job
				if err := job.Execute(); err != nil {
					log.Printf("Worker job error: %v", err)
				}
			case <-w.stop:
				return
			}
		}
	}()
}

// Stop halts the worker's job processing
func (w *Worker) Stop() {
	close(w.stop)
	w.wg.Wait()
	close(w.jobQueue)
	close(w.workerPool)
}
