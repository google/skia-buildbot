// Command line app to process fiddles in bulk.
//
// Example:
//  fiddlecli --input demo/testbulk.json --output /tmp/output.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"go.skia.org/infra/fiddlek/go/client"
	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/sync/errgroup"
)

const (
	// VERSION of the application. Update for major and minor changes to functionality.
	VERSION = "1.2"
)

// flags
var (
	domain   = flag.String("domain", "https://fiddle.skia.org", "Where to send the JSON request.")
	failFast = flag.Bool("fail_fast", false, "If true then exit with error on first send failure.")
	force    = flag.Bool("force", false, "Force a compile and run for each fiddle, don't take the fast path.")
	input    = flag.String("input", "fiddle.json", "The name of the file to read the JSON from.")
	output   = flag.String("output", "fiddleout.json", "The name of the file to write the JSON results to.")
	procs    = flag.Int("procs", 4, "The number of parallel requests to make to the fiddle server.")
	quiet    = flag.Bool("quiet", false, "Run without a progress bar.")
	version  = flag.Bool("version", false, "If true then echo the version number and exit.")
)

// chanRequest is sent to each worker in the pool.
type chanRequest struct {
	id  string
	req *types.FiddleContext
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
	if *version {
		fmt.Printf("fiddlecli version: %s\n", VERSION)
		os.Exit(0)
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
				b, err = json.Marshal(req.req)
				if err != nil {
					sklog.Errorf("Failed to encode an individual request: %s", err)
					continue
				}
				runResults, success := client.Do(b, *failFast, *domain, func(runResults *types.RunResults) bool {
					if fiddleHash != runResults.FiddleHash {
						sklog.Warningf("Got mismatched hashes for %s: Want %q != Got %q", req.id, fiddleHash, runResults.FiddleHash)
						return false
					}
					return true
				})
				if !success {
					sklog.Errorf("Failed to make request after retries")
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
		resp, ok := response[k]
		if !ok {
			sklog.Fatalf("Failed to get any response for %q", k)
		} else if hash != resp.FiddleHash {
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
