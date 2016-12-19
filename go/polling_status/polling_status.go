package polling_status

import (
	"encoding/json"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
)

// PollingStatus is a convenience struct used for periodically querying
// some resource.
type PollingStatus struct {
	lock   sync.RWMutex
	value  interface{}
	pollFn func() (interface{}, error)
	stop   chan bool
}

func NewPollingStatus(poll func() (interface{}, error), frequency time.Duration) *PollingStatus {
	s := PollingStatus{
		pollFn: poll,
		stop:   make(chan bool),
	}
	go func(s *PollingStatus) {
		ticker := time.Tick(frequency)
		for {
			select {
			case <-s.stop:
				return
			case <-ticker:
				if err := s.poll(); err != nil {
					sklog.Error(err)
				}
			}
		}
	}(&s)
	return &s
}

func (s *PollingStatus) poll() error {
	v, err := s.pollFn()
	if err != nil {
		return err
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.value = v
	return nil
}

func (s *PollingStatus) MarshalJSON() ([]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return json.Marshal(s.value)
}

func (s *PollingStatus) Stop() {
	s.stop <- true
}
