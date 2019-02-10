package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	RETRIES = 2
)

// singleRequest does a single request to run a fiddle
//
// c - HTTP client.
// body - The body to send to the fiddle /_/run endpoint.
// domain - The scheme and domain name to make requests to, e.g. "https://fiddle.skia.org".
// sleep - The duration to sleep if the request fails.
// failFast - If true then fail fatally.
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
		body := "(no body found)"
		b, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			body = string(b)
		}
		sklog.Infof("Send failed %q: %s", body, resp.Status)
		time.Sleep(sleep)
		return nil, false
	}
	var runResults types.RunResults
	if err := json.NewDecoder(resp.Body).Decode(&runResults); err != nil {
		sklog.Infof("Malformed response: %s", err)
		time.Sleep(sleep)
		return nil, false
	}
	// Occasionally runs will exceed 20s which looks like a security violation,
	// so this forces them to be re-run.
	if runResults.RunTimeError != "" {
		return nil, false
	}
	return &runResults, true
}

// Do runs a single fiddle contained in 'body'.
//
// failFast - If true then fail fatally.
// domain - The scheme and domain name to make requests to, e.g. "https://fiddle.skia.org".
// validator - A function that does extra validation on the fiddle run results. Return true if the
//   response is valid.
func Do(body []byte, failFast bool, domain string, validator func(*types.RunResults) bool) (*types.RunResults, bool) {
	c := httputils.NewTimeoutClient()
	success := false
	sleep := time.Second
	var runResults *types.RunResults
	for tries := 0; tries < RETRIES; tries++ {
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
