package util

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/skia-dev/glog"
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

// ReportError formats an HTTP error response and also logs the detailed error message.
func ReportError(w http.ResponseWriter, r *http.Request, err error, message string) {
	glog.Errorln(message, err)
	w.Header().Set("Content-Type", "text/plain")
	http.Error(w, fmt.Sprintf("%s %s", message, err), 500)
}
