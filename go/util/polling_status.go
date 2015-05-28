package util

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/skia-dev/glog"
)

// PollingStatus is a convenience struct used for periodically querying
// some resource.
type PollingStatus struct {
	lock   sync.RWMutex
	value  interface{}
	pollFn func(interface{}) error
	stop   chan bool
}

func NewPollingStatus(value interface{}, poll func(interface{}) error, frequency time.Duration) (*PollingStatus, error) {
	s := PollingStatus{
		sync.RWMutex{},
		value,
		poll,
		make(chan bool),
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
	if err := s.pollFn(s.value); err != nil {
		return err
	}
	return nil
}

func (s *PollingStatus) WriteJson(w io.Writer) error {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return json.NewEncoder(w).Encode(s.value)
}

func (s *PollingStatus) Stop() {
	s.stop <- true
}

// IntPollingStatus is a wrapper around PollingStatus which expects an
// integer value.
type IntPollingStatus struct {
	*PollingStatus
}

type intValue struct {
	Value int `json:"value" influxdb:"value"`
}

func NewIntPollingStatus(poll func(interface{}) error, frequency time.Duration) (*IntPollingStatus, error) {
	var val intValue
	s, err := NewPollingStatus(&val, poll, frequency)
	if err != nil {
		return nil, err
	}
	return &IntPollingStatus{s}, nil
}

func (s *IntPollingStatus) Get() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.value.(*intValue).Value
}

// JSONPollingStatus is a wrapper around PollingStatus which makes HTTP
// requests and expects responses which contain valid JSON. If client is nil,
// uses a default http.Client.
func NewJSONPollingStatus(value interface{}, url string, frequency time.Duration, client *http.Client) (*PollingStatus, error) {
	if client == nil {
		client = NewTimeoutClient()
	}
	return NewPollingStatus(value, func(interface{}) error {
		resp, err := client.Get(url)
		if err != nil {
			return err
		}
		defer Close(resp.Body)
		if err := json.NewDecoder(resp.Body).Decode(value); err != nil {
			return err
		}
		return nil
	}, frequency)
}
