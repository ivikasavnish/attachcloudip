package worker

import (
	"fmt"
	"sync"
)

// Job represents a task to be executed by a worker
type Job interface {
	Execute() error
}

// Dispatcher manages a pool of workers and a job queue
type Dispatcher struct {
	jobQueue   chan Job
	workerPool chan chan Job
	workers    []*Worker
	maxWorkers int
	stop       chan struct{}
	wg         sync.WaitGroup
}

// NewDispatcher creates a new Dispatcher with specified number of workers and queue size
func NewDispatcher(maxWorkers, queueSize int) *Dispatcher {
	return &Dispatcher{
		jobQueue:   make(chan Job, queueSize),
		workerPool: make(chan chan Job, maxWorkers),
		maxWorkers: maxWorkers,
		stop:       make(chan struct{}),
	}
}

// Start initializes and starts the worker pool
func (d *Dispatcher) Start() {
	// Create workers
	for i := 0; i < d.maxWorkers; i++ {
		worker := NewWorker(d.workerPool)
		d.workers = append(d.workers, worker)
		worker.Start()
	}

	// Start dispatcher routine
	d.wg.Add(1)
	go d.dispatch()
}

// dispatch distributes jobs to available workers
func (d *Dispatcher) dispatch() {
	defer d.wg.Done()

	for {
		select {
		case job := <-d.jobQueue:
			// Get an available worker
			workerJobQueue := <-d.workerPool
			// Send job to the worker
			workerJobQueue <- job
		case <-d.stop:
			return
		}
	}
}

// Submit adds a job to the job queue
func (d *Dispatcher) Submit(job Job) error {
	select {
	case d.jobQueue <- job:
		return nil
	default:
		return fmt.Errorf("job queue is full")
	}
}

// Stop gracefully shuts down the dispatcher and workers
func (d *Dispatcher) Stop() {
	close(d.stop)
	
	// Stop all workers
	for _, worker := range d.workers {
		worker.Stop()
	}

	// Wait for dispatcher to finish
	d.wg.Wait()

	// Close channels
	close(d.jobQueue)
	close(d.workerPool)
}
