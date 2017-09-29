package workerpool

/*
   Simple worker pool implementation.
*/

import "sync"

// WorkerPool is a struct used for managing a pool of worker goroutines.
type WorkerPool struct {
	mtx     sync.Mutex
	queue   chan func()
	size    int
	wg      sync.WaitGroup
	working bool
}

// Return a WorkerPool instance with the given number of worker goroutines.
func New(size int) *WorkerPool {
	p := &WorkerPool{
		size: size,
	}
	p.Reset()
	return p
}

// Reset spins up the worker goroutines and creates a new work queue so that the
// WorkerPool can be reused after a call to Close() or Wait().
func (p *WorkerPool) Reset() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if p.working {
		panic("Cannot Reset() a WorkerPool without first calling Wait()")
	}
	p.queue = make(chan func())
	p.wg = sync.WaitGroup{}
	p.working = true
	for i := 0; i < p.size; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for fn := range p.queue {
				fn()
			}
		}()
	}
}

// Go enqueues the given function onto the queue. Blocks until a worker is
// available to run the function. Panics if Wait() has already been called.
func (p *WorkerPool) Go(fn func()) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if !p.working {
		panic("Cannot enqueue work on a WorkerPool after Wait() without first calling Reset().")
	}
	p.queue <- fn
}

// Wait closes the queue and waits until all workers are finished. The
// WorkerPool cannot be reused again without a call to Reset().
func (p *WorkerPool) Wait() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if !p.working {
		panic("Cannot call Wait() on a WorkerPool more than once without first calling Reset().")
	}
	close(p.queue)
	p.working = false
	p.wg.Wait()
}
