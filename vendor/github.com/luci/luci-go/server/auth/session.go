// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package auth

import (
	"time"

	"golang.org/x/net/context"
)

// SessionStore keeps user sessions in some permanent storage. SessionStore is
// used by some authentication methods (e.g. openid.AuthMethod).
type SessionStore interface {
	// OpenSession create a new session for a user with given expiration time.
	// It returns unique session ID.
	OpenSession(c context.Context, userID string, u *User, exp time.Time) (string, error)

	// CloseSession closes a session given its ID. Does nothing if session is
	// already closed or doesn't exist. Returns only transient errors.
	CloseSession(c context.Context, sessionID string) error

	// GetSession returns existing non-expired session given its ID. Returns nil
	// if session doesn't exist, closed or expired. Returns only transient errors.
	GetSession(c context.Context, sessionID string) (*Session, error)
}

// Session is returned by SessionStore.GetSession(...).
type Session struct {
	SessionID string    // same as `sessionID` passed to GetSession()
	UserID    string    // authentication provider specific user id
	User      User      // user profile, including identity string
	Exp       time.Time // when the session expires
}
