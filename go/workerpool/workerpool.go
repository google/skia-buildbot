package workerpool

/*
   Simple worker pool implementation.
*/

import "sync"

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
// available to run the function. Panics if Wait() has already been called.
func (p *WorkerPool) Go(fn func()) {
	p.queue <- fn
}

// Wait closes the queue and waits until all workers are finished. The
// WorkerPool cannot be reused again.
func (p *WorkerPool) Wait() {
	close(p.queue)
	p.wg.Wait()
}
