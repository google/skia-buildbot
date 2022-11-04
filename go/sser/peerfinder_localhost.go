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
