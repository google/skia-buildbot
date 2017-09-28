package workerpool

import "sync"

/*
	Simple worker pool implementation.
*/

// WorkerPool is a struct used for managing a pool of worker goroutines.
type WorkerPool struct {
	queue chan func()
	wg    sync.WaitGroup
}

// Return a WorkerPool instance with the given number of worker goroutines.
func New(size int) *WorkerPool {
	p := &WorkerPool{
		queue: make(chan func()),
		wg:    sync.WaitGroup{},
	}
	for i := 0; i < size; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for fn := range p.queue {
				fn()
			}
		}()
	}
	return p
}

// Go enqueues the given function onto the queue. Blocks until a worker is
// available to run the function.
func (p *WorkerPool) Go(fn func()) {
	p.queue <- fn
}

// Close closes the queue channel, ending the worker goroutines. Calls to Go
// after a call to Close will result in a panic.
func (p *WorkerPool) Close() {
	close(p.queue)
}

// Wait closes the queue and waits until all workers are finished.
func (p *WorkerPool) Wait() {
	p.Close()
	p.wg.Wait()
}
