// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package authtest

import (
	"fmt"
	"sync"
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/server/auth"
	"golang.org/x/net/context"
)

// MemorySessionStore implement auth.SessionStore.
type MemorySessionStore struct {
	lock    sync.Mutex
	store   map[string]auth.Session
	counter int
}

// OpenSession create a new session for a user with given expiration time.
// It returns unique session ID.
func (s *MemorySessionStore) OpenSession(c context.Context, userID string, u *auth.User, exp time.Time) (string, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.counter++
	if s.store == nil {
		s.store = make(map[string]auth.Session, 1)
	}
	sid := fmt.Sprintf("%s/%d", userID, s.counter)
	s.store[sid] = auth.Session{
		SessionID: sid,
		UserID:    userID,
		User:      *u,
		Exp:       exp,
	}
	return sid, nil
}

// CloseSession closes a session given its ID. Does nothing if session is
// already closed or doesn't exist. Returns only transient errors.
func (s *MemorySessionStore) CloseSession(c context.Context, sessionID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.store, sessionID)
	return nil
}

// GetSession returns existing non-expired session given its ID. Returns nil
// if session doesn't exist, closed or expired. Returns only transient errors.
func (s *MemorySessionStore) GetSession(c context.Context, sessionID string) (*auth.Session, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if session, ok := s.store[sessionID]; ok && clock.Now(c).Before(session.Exp) {
		return &session, nil
	}
	return nil, nil
}
