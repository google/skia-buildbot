package rtcache

import (
	"container/heap"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	ttlcache "github.com/patrickmn/go-cache"
)

// TODO(stephana): Add the ability to purge items from the cache and
// expunge item from the error cache. Remove DEFAULT_CACHESIZE when
// we have a way to expung items from the cache.

const (
	// Duration to cache an error response.
	DEFAULT_ERRCACHE_EXPIRATION_TIME = time.Minute * 30

	// Interval at which the errcache is cleared of expired entries.
	ERRCACHE_CLEANUP_TIME = time.Minute * 5
)

// MemReadThroughCache implements the ReadThroughCache interface.
type MemReadThroughCache struct {
	workerFn       ReadThroughFunc      // worker function to create the items.
	cache          *lru.Cache           // caches the items in RAM.
	errCache       *ttlcache.Cache      // caches errors for a limited time.
	pQ             *priorityQueue       // priority queue to order item generation.
	pqItemLookup   map[string]*workItem // lookup items by id in pQ.
	inProgress     map[string]*workItem // items that are currently being generated.
	mutex          sync.Mutex           // protecs all members of this instance.
	emptyCond      *sync.Cond           // used to synchronize workers when the queue is empty.
	finishedCh     chan bool            // closing this signals go-routines to shut down.
	wg             sync.WaitGroup       // allows to synchronize go-routines during shutdown.
	activeWorkerCh chan bool            // records the workers that are currently running.
}

// New returns a new instance of ReadThroughCache that is stored in RAM.
// nWorkers defines the number of concurrent workers that call wokerFn when
// requested items are not in RAM.
func New(workerFn ReadThroughFunc, maxSize int, nWorkers int) (ReadThroughCache, error) {
	// if maxSize is <= 0 then we don't cache at all. But lru.Cache will not
	// limit the cache if the size is 0. So we cache 1 element.
	if maxSize <= 0 {
		maxSize = 1
	}

	lruCache, err := lru.New(maxSize)
	if err != nil {
		return nil, err
	}

	ret := &MemReadThroughCache{
		workerFn:       workerFn,
		cache:          lruCache,
		errCache:       ttlcache.New(DEFAULT_ERRCACHE_EXPIRATION_TIME, ERRCACHE_CLEANUP_TIME),
		pQ:             &priorityQueue{},
		inProgress:     map[string]*workItem{},
		pqItemLookup:   map[string]*workItem{},
		finishedCh:     make(chan bool),
		activeWorkerCh: make(chan bool, nWorkers),
	}
	ret.emptyCond = sync.NewCond(&ret.mutex)
	ret.startWorker()
	return ret, nil
}

// Get implements the ReadThroughCache interface.
func (m *MemReadThroughCache) Get(priority int64, id string) (interface{}, error) {
	ret, err, resultCh := m.getOrEnqueue(priority, id)
	if (err != nil) || (ret != nil) {
		return ret, err
	}

	// Wait for the result.
	ret = <-resultCh
	if err, ok := ret.(error); ok {
		return nil, err
	}
	return ret, nil
}

// Keys implements the ReadThroughCache interface.
func (m *MemReadThroughCache) Keys() []string {
	m.mutex.Lock()
	keys := m.cache.Keys()
	m.mutex.Unlock()

	// Convert to strings.
	ret := make([]string, len(keys))
	for idx, key := range keys {
		ret[idx] = key.(string)
	}
	return ret
}

// Remove implements the ReadThroughCache interface.
func (m *MemReadThroughCache) Remove(ids []string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, id := range ids {
		m.cache.Remove(id)
	}
}

// getOrEnqueue retrieves the desired item from the cache or schedules that it be calculated.
// The returned channel can then be used to wait for the result.
func (m *MemReadThroughCache) getOrEnqueue(priority int64, id string) (interface{}, error, chan interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if it's in the cache.
	if result, ok := m.cache.Get(id); ok {
		return result, nil, nil
	}

	// Check if it's in the error cache.
	if err, ok := m.errCache.Get(id); ok {
		return nil, err.(error), nil
	}

	// Check if it's in already in progress, if not add it to the work queue.
	resultCh := make(chan interface{})
	if wi, ok := m.inProgress[id]; ok {
		wi.resultChans = append(wi.resultChans, resultCh)
	} else {
		m.enqueue(id, priority, resultCh)
	}
	return nil, nil, resultCh
}

// enqueue adds to given item to the priority queue. This assumes that the
// caller currently holds the mutex.
func (m *MemReadThroughCache) enqueue(id string, priority int64, resultCh chan interface{}) {
	// if the items exists then update the itme.
	if found, ok := m.pqItemLookup[id]; ok {
		found.resultChans = append(found.resultChans, resultCh)
		if found.priority != priority {
			found.priority = priority
			heap.Fix(m.pQ, found.idx)
		}
		return
	}

	item := &workItem{
		id:          id,
		priority:    priority,
		resultChans: []chan interface{}{resultCh},
	}
	heap.Push(m.pQ, item)
	m.pqItemLookup[id] = item
	m.emptyCond.Signal()
}

// dequeue returns the next workItem. It blocks until an item is available.
// The caller must NOT hold the mutex when calling. Moves the found item
// to inProgres table. If the finishedCh is closed this function will
// return nil.
func (m *MemReadThroughCache) dequeue() *workItem {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for len(*m.pQ) == 0 {
		if m.finished() {
			return nil
		}
		m.emptyCond.Wait()
	}

	ret := heap.Pop(m.pQ).(*workItem)
	delete(m.pqItemLookup, ret.id)
	m.inProgress[ret.id] = ret
	return ret
}

// saveResult stores the given result in the cache. It also notifies the
// any waiting calls to Get(...) that the results are ready.
func (m *MemReadThroughCache) saveResult(wi *workItem, result interface{}, err error) {
	m.mutex.Lock()

	if err != nil {
		m.errCache.Set(wi.id, err, DEFAULT_ERRCACHE_EXPIRATION_TIME)
		result = err
	} else {
		m.cache.Add(wi.id, result)
	}

	delete(m.inProgress, wi.id)
	m.mutex.Unlock()

	for _, ch := range wi.resultChans {
		ch <- result
	}
}

// finished returns true if the finishedCh was closed. Indicating that
// all go-routines should shut down.
func (m *MemReadThroughCache) finished() bool {
	select {
	case <-m.finishedCh:
		return true
	default:
	}
	return false
}

// Terminates all go routines and waits until they terminate. Used for testing.
func (m *MemReadThroughCache) shutdown() {
	close(m.finishedCh)
	m.emptyCond.Broadcast()
	m.wg.Wait()
}

// startWorker starts a background process that calculates cache values when
// requested while not exceeding the configured number of workers.
func (m *MemReadThroughCache) startWorker() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			// Allocate a slot int he active workers channel.
			select {
			case <-m.finishedCh:
				return
			case m.activeWorkerCh <- true:
			}

			wi := m.dequeue()
			if wi != nil {
				// Start a go-routine to calculate the task.
				m.wg.Add(1)
				go func(wi *workItem) {
					defer m.wg.Done()
					ret, err := m.workerFn(wi.priority, wi.id)
					m.saveResult(wi, ret, err)
					<-m.activeWorkerCh
				}(wi)
			}
		}
	}()
}

// Warm implements the ReadThroughCache interface.
func (m *MemReadThroughCache) Warm(priority int64, id string) error {
	_, err := m.Get(priority, id)
	return err
}

// Contains implements the ReadThroughCache interface.
func (m *MemReadThroughCache) Contains(id string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	_, ok := m.cache.Get(id)
	return ok
}

// workItem is used to control calls to workerFn when an item is not
// in memory. The priority field defines it's position in the priority
// queueu.
type workItem struct {
	id          string             // id of the item that needs to be retrieved.
	idx         int                // index of the item in the priority queue.
	priority    int64              // priority of the item.
	resultChans []chan interface{} // waiting Get(...) calls that need to be notified.
}

// priorityQueue implements heap.Interface.
type priorityQueue []*workItem

// implement the sort.Interface portion of heap.Interface.
func (p *priorityQueue) Len() int           { return len(*p) }
func (p *priorityQueue) Less(i, j int) bool { return (*p)[i].priority < (*p)[j].priority }
func (p *priorityQueue) Swap(i, j int) {
	(*p)[i], (*p)[j] = (*p)[j], (*p)[i]
	(*p)[i].idx = i
	(*p)[j].idx = j
}

// Push implements heap.Interface.
func (p *priorityQueue) Push(x interface{}) {
	item := x.(*workItem)
	item.idx = len(*p)
	*p = append(*p, item)
}

// Push implements heap.Interface.
func (p *priorityQueue) Pop() interface{} {
	n := len(*p)
	ret := (*p)[n-1]
	*p = (*p)[:n-1]
	return ret
}
