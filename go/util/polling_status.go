package util

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/skia-dev/glog"
)

// PollingStatus is a convenience struct used for periodically querying
// some resource.
type PollingStatus struct {
	lock   sync.RWMutex
	value  interface{}
	pollFn func() (interface{}, error)
	stop   chan bool
}

func NewPollingStatus(poll func() (interface{}, error), frequency time.Duration) (*PollingStatus, error) {
	s := PollingStatus{
		pollFn: poll,
		stop:   make(chan bool),
	}
	if err := s.poll(); err != nil {
		return nil, err
	}
	go func(s *PollingStatus) {
		ticker := time.Tick(frequency)
		for {
			select {
			case <-s.stop:
				return
			case <-ticker:
				if err := s.poll(); err != nil {
					glog.Error(err)
				}
			}
		}
	}(&s)
	return &s, nil
}

func (s *PollingStatus) poll() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	v, err := s.pollFn()
	if err != nil {
		return err
	}
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
