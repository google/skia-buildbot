// Command line app to process fiddles in bulk.
//
// Example:
//  fiddlecli --input demo/testbulk.json --output /tmp/output.json
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	RETRIES = 10
)

// flags
var (
	domain   = flag.String("domain", "https://fiddle.skia.org", "Where to send the JSON request.")
	failFast = flag.Bool("fail_fast", false, "If true then exit with error on first send failure.")
	force    = flag.Bool("force", false, "Force a compile and run for each fiddle, don't take the fast path.")
	input    = flag.String("input", "", "The name of the file to read the JSON from.")
	output   = flag.String("output", "", "The name of the file to write the JSON results to.")
	procs    = flag.Int("procs", 4, "The number of parallel requests to make to the fiddle server.")
	quiet    = flag.Bool("quiet", false, "Run without a progress bar.")
)

// chanRequest is sent to each worker in the pool.
type chanRequest struct {
	id  string
	req *types.FiddleContext
}

// singleRequest does a single request to fiddle.skia.org.
func singleRequest(c *http.Client, body []byte, domain string) (*types.RunResults, bool) {
	resp, err := c.Post(domain+"/_/run", "application/json", bytes.NewReader(body))
	sleep := time.Second
	if err != nil {
		sklog.Infof("Send error: %s", err)
		time.Sleep(sleep)
		return nil, false
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		if *failFast {
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

func main() {
	// Check flags.
	common.Init()
	if *input == "" {
		flag.Usage()
		log.Fatalf("--input is a required flag.")
	}
	if *output == "" {
		flag.Usage()
		log.Fatalf("--output is a required flag.")
	}

	// Read the source JSON file.
	b, err := ioutil.ReadFile(*input)
	if err != nil {
		log.Fatalf("Failed to read %s: %s", *input, err)
	}
	requests := types.BulkRequest{}
	if err := json.Unmarshal(b, &requests); err != nil {
		log.Fatalf("%s does not contain valid JSON: %s", *input, err)
	}
	lastWritten := types.BulkResponse{}
	b, err = ioutil.ReadFile(*output)
	if err == nil {
		if err := json.Unmarshal(b, &lastWritten); err != nil {
			lastWritten = nil
		}
	}
	g := errgroup.Group{}
	requestsCh := make(chan chanRequest, len(requests))

	// mutex protects response.
	mutex := sync.Mutex{}
	response := types.BulkResponse{}

	// Spin up workers.
	for i := 0; i < *procs; i++ {
		g.Go(func() error {
			for req := range requestsCh {
				if !*quiet {
					fmt.Print(".")
				}
				fiddleHash, err := req.req.Options.ComputeHash(req.req.Code)
				if err != nil {
					sklog.Fatalf("Failed to calculate fiddleHash: %s", err)
				}
				if *force {
					req.req.Fast = false
				} else if lastWritten != nil && fiddleHash != "" && lastWritten[req.id] != nil && lastWritten[req.id].FiddleHash == fiddleHash {
					mutex.Lock()
					response[req.id] = lastWritten[req.id]
					mutex.Unlock()
					continue
				}
				success := false
				var runResults *types.RunResults
				for tries := 0; tries < RETRIES; tries++ {
					c := httputils.NewTimeoutClient()
					// POST to fiddle.
					b, err = json.Marshal(req.req)
					if err != nil {
						sklog.Errorf("Failed to encode an individual request: %s", err)
						break
					}
					runResults, success = singleRequest(c, b, *domain)
					if success {
						if fiddleHash != runResults.FiddleHash {
							sklog.Warningf("Got mismatched hashes for %s: Want %q != Got %q", req.id, fiddleHash, runResults.FiddleHash)
						} else {
							break
						}
					}
					fmt.Print("x")
				}
				if !success {
					sklog.Errorf("Failed to make request after %d tries", RETRIES)
					continue
				}

				mutex.Lock()
				response[req.id] = runResults
				mutex.Unlock()
			}
			return nil
		})
	}

	// Loop over each entry and queue them up for the workers.
	for id, req := range requests {
		requestsCh <- chanRequest{
			id:  id,
			req: req,
		}
	}
	close(requestsCh)

	// Wait for the workers to finish.
	if err := g.Wait(); err != nil {
		log.Fatalf("Failed to complete all requests: %s", err)
	}

	// Validate the output.
	for k, v := range requests {
		hash, _ := v.Options.ComputeHash(v.Code)
		if hash != response[k].FiddleHash {
			sklog.Fatalf("For %q want %q but got %q", k, hash, response[k].FiddleHash)
		}
	}

	if !*quiet {
		fmt.Print("\n")
	}
	b, err = json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatalf("Failed to encode response file: %s", err)
	}
	if err := ioutil.WriteFile(*output, b, 0600); err != nil {
		log.Fatalf("Failed to write response file: %s", err)
	}
}
