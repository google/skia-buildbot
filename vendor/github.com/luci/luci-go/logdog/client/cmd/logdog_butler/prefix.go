// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os/user"

	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

const (
	// userPrefixBytes is the size, in bytes, of the user prefix random component.
	userPrefixBytes = 30
)

// getCurrentUser returns the current user name.
//
// It's a variable so that it can be overridden for testing.
var getCurrentUser = func() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

// generateRandomPrefix generates a new log prefix in the "user" space.
func generateRandomUserPrefix(ctx context.Context) (types.StreamName, error) {
	username, err := getCurrentUser()
	if err != nil {
		log.Warningf(log.SetError(ctx, err), "Failed to lookup current user name.")
		username = "user"
	}

	buf := make([]byte, userPrefixBytes)
	c, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	if c != len(buf) {
		return "", errors.New("main: failed to fill user prefix bytes")
	}

	streamName, err := types.MakeStreamName("s_", "butler", "users", username, base64.URLEncoding.EncodeToString(buf))
	if err != nil {
		return "", fmt.Errorf("failed to generate user prefix: %s", err)
	}
	return streamName, nil
}
