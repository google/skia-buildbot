// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sser

import "context"

// PeerFinderLocalhost implements PeerFinder but only finds "127.0.0.1".
type PeerFinderLocalhost struct{}

// Start implements PeerFinder.
func (p PeerFinderLocalhost) Start(ctx context.Context) ([]string, <-chan []string, error) {
	return []string{"127.0.0.1"}, make(chan []string), nil
}

func NewPeerFinderLocalhost() PeerFinderLocalhost {
	return PeerFinderLocalhost{}
}

// Confirm PeerFinderLocalhost implements PeerFinder.
var _ PeerFinder = PeerFinderLocalhost{}
