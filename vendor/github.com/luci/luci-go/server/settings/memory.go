// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"encoding/json"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
)

// MemoryStorage implements Storage interface, using memory as a backend. Useful
// in unit tests.
type MemoryStorage struct {
	Expiration time.Duration // default expiration time of in-memory cache

	lock   sync.Mutex
	values map[string]*json.RawMessage
}

// FetchAllSettings fetches all latest settings at once.
func (m *MemoryStorage) FetchAllSettings(c context.Context) (*Bundle, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	values := make(map[string]*json.RawMessage, len(m.values))
	for k, v := range m.values {
		if v == nil {
			values[k] = v
		} else {
			raw := append(json.RawMessage(nil), *v...)
			values[k] = &raw
		}
	}
	return &Bundle{Values: values, Exp: clock.Now(c).Add(m.Expiration)}, nil
}

// UpdateSetting updates a setting at the given key.
func (m *MemoryStorage) UpdateSetting(c context.Context, key string, value json.RawMessage, who, why string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	if m.values == nil {
		m.values = make(map[string]*json.RawMessage, 1)
	}
	cpy := append(json.RawMessage(nil), value...)
	m.values[key] = &cpy
	return nil
}
