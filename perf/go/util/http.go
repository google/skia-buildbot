package util

import (
	"net"
	"net/http"
	"time"
)

const (
	// TIMEOUT is the http timeout when making requests.
	TIMEOUT = time.Duration(time.Minute)
)

// DialTimeout is a dialer that sets a timeout.
func DialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, TIMEOUT)
}

// NewTimeoutClient creates a new http.Client with a timeout.
func NewTimeoutClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: DialTimeout,
		},
	}
}
