package progress

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"

	"go.skia.org/infra/go/now"
)

var (
	// newTimeTickerFunc allows overriding time.NewTicker for testing.
	newTimeTickerFunc now.NewTimeTickerFunc = now.NewTimeTicker

	// callbackFunc is the function which report the number of bytes transferred.
	callbackFunc = loggingCallbackFunc
)

// loggingCallbackFunc logs the number of transferred bytes.
func loggingCallbackFunc(byteCount int64) {
	fmt.Println(fmt.Sprintf("%s transferred", humanize.Bytes(uint64(byteCount))))
}

// readCloser pairs an io.Reader and an io.Closer to form an io.ReadCloser.
type readCloser struct {
	io.Reader
	io.Closer
}

var _ io.ReadCloser = readCloser{}

// callbackReader is an io.Reader which calls a function whenever bytes are
// read.
type callbackReader struct {
	io.Reader
	cb func(int)
}

// Read implements io.Reader.
func (r *callbackReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.cb(n)
	return n, err
}

// newCallbackReader returns an io.Reader which calls the given callback
// function whenever bytes are written.
func newCallbackReader(r io.Reader, cb func(int)) *callbackReader {
	return &callbackReader{
		Reader: r,
		cb:     cb,
	}
}

var _ io.Reader = &callbackReader{}

// ProgressTracker tracks the number of bytes which have been transferred and
// periodically logs the number to stdout.
type ProgressTracker struct {
	ch     chan int
	enable chan bool
}

// track is called by ProgressTrackingRoundTripper whenever bytes are
// transferred.
func (t *ProgressTracker) track(n int) {
	t.ch <- n
}

// Start tracking the number of bytes transferred.
func (t *ProgressTracker) Start() {
	t.enable <- true
}

// Stop tracking the number of bytes transferred.
func (t *ProgressTracker) Stop() {
	t.enable <- false
}

// NewProgressTracker returns a ProgressTracker.
func NewProgressTracker() *ProgressTracker {
	t := &ProgressTracker{
		ch:     make(chan int),
		enable: make(chan bool),
	}
	var callback func(int64) = nil
	count := int64(0)
	ticker := newTimeTickerFunc(time.Second)

	go func() {
		for {
			select {
			case n := <-t.ch:
				count += int64(n)
			case <-ticker.C():
				if callback != nil {
					callback(count)
				}
			case enable := <-t.enable:
				if enable {
					callback = callbackFunc
				} else {
					callback = nil
				}
				count = 0
			}
		}
	}()
	return t
}

// ProgressTrackingRoundTripper is a http.RoundTripper which wraps another
// http.RoundTripper and prints information about the number of bytes which have
// been transferred.
type ProgressTrackingRoundTripper struct {
	http.RoundTripper
	upload   *ProgressTracker
	download *ProgressTracker
}

// RoundTrip implements http.RoundTripper.
func (t *ProgressTrackingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		req.Body = readCloser{
			Reader: newCallbackReader(req.Body, t.upload.track),
			Closer: req.Body,
		}
	}
	resp, err := t.RoundTripper.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		resp.Body = readCloser{
			Reader: newCallbackReader(resp.Body, t.download.track),
			Closer: resp.Body,
		}
	}
	return resp, err
}

// NewProgressTrackingRoundTripper returns a http.RoundTripper which wraps the
// given http.RoundTripper and prints information about the number of bytes
// which have been transferred.
func NewProgressTrackingRoundTripper(wrap http.RoundTripper) (*ProgressTrackingRoundTripper, *ProgressTracker, *ProgressTracker) {
	if wrap == nil {
		wrap = http.DefaultTransport
	}
	upload := NewProgressTracker()
	download := NewProgressTracker()
	return &ProgressTrackingRoundTripper{
		RoundTripper: wrap,
		upload:       upload,
		download:     download,
	}, upload, download
}

var _ http.RoundTripper = &ProgressTrackingRoundTripper{}

// ProgressTrackingClient returns a http.Client which wraps the given
// http.Client and prints information about the number of bytes which have been
// transferred.  Use the returned upload and download ProgressTrackers to start
// and stop tracking transfers.
func ProgressTrackingClient(wrap *http.Client) (client *http.Client, upload *ProgressTracker, download *ProgressTracker) {
	transport, upload, download := NewProgressTrackingRoundTripper(wrap.Transport)
	return &http.Client{
		Transport:     transport,
		CheckRedirect: wrap.CheckRedirect,
		Jar:           wrap.Jar,
		Timeout:       wrap.Timeout,
	}, upload, download
}
