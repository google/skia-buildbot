package influxdb

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/skia-dev/glog"
)

// PollingStatus is a convenience struct used for periodically querying
// InfluxDB.
type PollingStatus struct {
	lock     sync.RWMutex
	value    interface{}
	query    string
	dbClient *Client
}

func NewPollingStatus(value interface{}, query string, dbClient *Client) (*PollingStatus, error) {
	s := PollingStatus{sync.RWMutex{}, value, query, dbClient}
	if err := s.poll(); err != nil {
		return nil, err
	}
	go func(s *PollingStatus) {
		for _ = range time.Tick(time.Minute) {
			if err := s.poll(); err != nil {
				glog.Errorf(err.Error())
			}
		}
	}(&s)
	return &s, nil
}

func (s *PollingStatus) poll() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.dbClient.Query(&s.value, s.query)
}

func (s *PollingStatus) WriteJson(w io.Writer) error {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return json.NewEncoder(w).Encode(s.value)
}

// IntPollingStatus is a wrapper around PollingStatus which expects an
// integer value as the InfluxDB query result.
type IntPollingStatus struct {
	*PollingStatus
}

type intValue struct {
	Value int `influxdb:"value"`
}

func NewIntPollingStatus(query string, client *Client) (*IntPollingStatus, error) {
	var val intValue
	s, err := NewPollingStatus(&val, query, client)
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
