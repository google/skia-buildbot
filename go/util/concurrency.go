package util

import "sync"

// CondMonitor implements a monitor that limits the number of threads
// per int64-id that can concurrently enter a critical section.
// This could be used like this:
//    ...
//    mon := NewCondInt64Monitor(5)
//    ...
//    defer mon.Enter(id).Release()
//    ...
//
// This allows 5 threads for each value of id to enter the critical section
// which starts after the call to 'Enter' and ends when 'Release' is called.
type CondMonitor struct {
	cond        sync.Cond
	countMap    map[int64]int
	nConcurrent int
}

// NewCondMonitor creates a new monitor with the given number of
// concurrent threads that can enter the critical section for every int64 value
// provided to the 'Enter' call.
func NewCondMonitor(nConcurrent int) *CondMonitor {
	return &CondMonitor{
		countMap:    map[int64]int{},
		nConcurrent: nConcurrent,
		cond:        sync.Cond{L: &sync.Mutex{}},
	}
}

// MonitorRelease is returned by the Enter call.
type MonitorRelease func()

// Release is called to identify the end of the critical section.
func (m MonitorRelease) Release() { m() }

// Enter marks the start of a critical section. It limits the number of
// threads
func (m *CondMonitor) Enter(id int64) MonitorRelease {
	// Get the lock over the entire map.
	m.cond.L.Lock()
	for {
		// If the requested id is below the concurrency threshold continue otherwise
		// wait until somebody leaves the monitor.
		if m.countMap[id] < m.nConcurrent {
			m.countMap[id]++
			break
		}
		m.cond.Wait()
	}
	m.cond.L.Unlock()

	// Return the function to leave the monitor.
	return func() {
		m.cond.L.Lock()
		// Decrement the counter and signal the waiting threads to reexamine the counter.
		m.countMap[id]--
		if m.countMap[id] == 0 {
			delete(m.countMap, id)
		}
		m.cond.L.Unlock()
		m.cond.Broadcast()
	}
}
