package progress

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/now/mocks"
)

func TestProgressTracker(t *testing.T) {
	// Override newTimeTickerFunc to supply our own ticks.
	tickerCh := make(chan time.Time)
	newTimeTickerFunc = mocks.NewTimeTickerFunc(tickerCh)

	// Override the callback function with one which loads byte counts into a
	// channel.
	countsCh := make(chan int64)
	callbackFunc = func(byteCount int64) {
		countsCh <- byteCount
	}

	// check sends a tick along the time channel and waits for the next byte
	// count to be received from the callback function, then verifies that it
	// equals the expected value.  Note that this will hang if the
	// ProgressTracker is not Started.
	check := func(expect int64) {
		tickerCh <- time.Time{}
		require.Equal(t, expect, <-countsCh)
	}
	p := NewProgressTracker()

	// We're not tracking progress yet, so this shouldn't do anything.
	p.track(1)

	// Start tracking progress. The byte count at the next tick should be zero.
	p.Start()
	check(0)

	// Send a 2, verify that we get it back on the next tick.
	p.track(2)
	check(2)

	// Send a 3 and 4, verify that we get the total back on the next tick.
	p.track(3)
	p.track(4)
	check(9)

	// Stop tracking progress.
	p.Stop()

	// Send a 10. We're no longer tracking progress, so this shouldn't get
	// recorded.
	p.track(10)

	// Start tracking progress again. We should get another zero at the next
	// tick.
	p.Start()
	check(0)

	// Verify that we're tracking progress again.
	p.track(1)
	p.track(10)
	p.track(100)
	check(111)
}

func TestProgressTrackingClient(t *testing.T) {
	// Override newTimeTickerFunc to supply our own ticks.
	tickerCh := make(chan time.Time)
	newTimeTickerFunc = mocks.NewTimeTickerFunc(tickerCh)

	// Override the callback function with one which loads byte counts into a
	// channel.
	countsCh := make(chan int64)
	callbackFunc = func(byteCount int64) {
		countsCh <- byteCount
	}

	// check sends a tick along the time channel and waits for the next byte
	// count to be received from the callback function, then verifies that it
	// equals the expected value.  Note that this will hang if the
	// ProgressTracker is not Started.
	check := func(expect int64) {
		// We have two ProgressTrackers and therefore two consumers of tickerCh,
		// so we have to send two ticks.
		tickerCh <- time.Time{}
		tickerCh <- time.Time{}
		require.Equal(t, expect, <-countsCh)
	}

	// Create the client.
	urlMock := mockhttpclient.NewURLMock()
	client, upload, download := ProgressTrackingClient(urlMock.Client())

	// Note: this is inherently racy; if the signal produced by Start() is not
	// consumed by the time we send a value onto the tickerCh, the goroutine
	// might consume either of those values first and the test may flakily hang.
	// Therefore, we Sleep a bit when we Start and Stop.
	download.Start()
	time.Sleep(100 * time.Millisecond)

	// Send a GET request, verify that we counted the response bytes.
	mockRespBytes := []byte("fake-response")
	urlMock.MockOnce("fake-get", mockhttpclient.MockGetDialogue(mockRespBytes))
	check(0)
	resp, err := client.Get("fake-get")
	require.NoError(t, err)
	actualRespBytes, err := ioutil.ReadAll(resp.Body)
	require.Equal(t, mockRespBytes, actualRespBytes)
	check(int64(len(mockRespBytes)))

	// Switch to tracking uploads.
	download.Stop()
	upload.Start()
	time.Sleep(100 * time.Millisecond) // Sleep for the reasons discussed above.

	// Send a POST request, verify that we counted the request bytes.
	mockReqBytes := []byte("fake-request")
	urlMock.MockOnce("fake-post", mockhttpclient.MockPostDialogue("application/json", mockReqBytes, mockRespBytes))
	check(0)
	resp, err = client.Post("fake-post", "application/json", bytes.NewReader(mockReqBytes))
	require.NoError(t, err)
	actualRespBytes, err = ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, mockRespBytes, actualRespBytes)
	check(int64(len(mockReqBytes)))
	upload.Stop()
}
