// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"encoding/json"
	"sync"
	"time"
)

// RecordedMutation captures a write request received by the mock server so E2E
// and unit tests can verify outgoing payloads without network interception.
type RecordedMutation struct {
	Endpoint  string          `json:"endpoint"`
	Method    string          `json:"method"`
	Timestamp int64           `json:"timestamp"`
	Body      json.RawMessage `json:"body"`
}

// MutationRecorder stores an ordered log of write requests across test steps.
type MutationRecorder struct {
	mu        sync.RWMutex
	mutations []RecordedMutation
}

func NewMutationRecorder() *MutationRecorder {
	return &MutationRecorder{
		mutations: []RecordedMutation{},
	}
}

func (r *MutationRecorder) Record(endpoint, method string, rawBody []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var body json.RawMessage
	if len(rawBody) > 0 && json.Valid(rawBody) {
		copied := make([]byte, len(rawBody))
		copy(copied, rawBody)
		body = json.RawMessage(copied)
	} else {
		body = json.RawMessage(`{}`)
	}

	r.mutations = append(r.mutations, RecordedMutation{
		Endpoint:  endpoint,
		Method:    method,
		Timestamp: time.Now().Unix(),
		Body:      body,
	})
}

func (r *MutationRecorder) List() []RecordedMutation {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]RecordedMutation, len(r.mutations))
	copy(out, r.mutations)
	return out
}

func (r *MutationRecorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mutations = []RecordedMutation{}
}
