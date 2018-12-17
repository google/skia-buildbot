package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	RETRIES = 10
)

// singleRequest does a single request to fiddle.skia.org.
func singleRequest(c *http.Client, body []byte, domain string, sleep time.Duration, failFast bool) (*types.RunResults, bool) {
	resp, err := c.Post(domain+"/_/run", "application/json", bytes.NewReader(body))
	if err != nil {
		sklog.Infof("Send error: %s", err)
		time.Sleep(sleep)
		return nil, false
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		if failFast {
			sklog.Fatalf("Send failed, with fail_fast set: %s", resp.Status)
		}
		sklog.Infof("Send failed: %s", resp.Status)
		time.Sleep(sleep)
		return nil, false
	}
	var runResults types.RunResults
	if err := json.NewDecoder(resp.Body).Decode(&runResults); err != nil {
		sklog.Infof("Malformed response: %s", err)
		time.Sleep(sleep)
		return nil, false
	}
	return &runResults, true
}

func Do(body []byte, failFast bool, domain string, validator func(*types.RunResults) bool) (*types.RunResults, bool) {
	c := httputils.NewTimeoutClient()
	success := false
	var runResults *types.RunResults
	for tries := 0; tries < RETRIES; tries++ {
		sleep := time.Second
		// POST to fiddle.
		runResults, success = singleRequest(c, body, domain, sleep, failFast)
		if success {
			if validator(runResults) {
				return runResults, success
			}
		}
		sleep *= 2
		fmt.Print("x")
	}
	return nil, false
}
