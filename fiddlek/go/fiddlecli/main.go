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
	"sync"

	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/httputils"
)

// flags
var (
	domain = flag.String("domain", "https://fiddle.skia.org", "Where to send the JSON request.")
	input  = flag.String("input", "", "The name of the file to read the JSON from.")
	output = flag.String("output", "", "The name of the file to write the JSON results to.")
	procs  = flag.Int("procs", 4, "The number of parallel requests to make to the fiddle server.")
	quiet  = flag.Bool("quiet", false, "Run without a progress bar.")
	force  = flag.Bool("force", false, "Force a compile and run for each fiddle, don't take the fast path.")
)

// chanRequest is sent to each worker in the pool.
type chanRequest struct {
	id  string
	req *types.FiddleContext
}

func main() {
	// Check flags.
	flag.Parse()
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

	c := httputils.NewTimeoutClient()
	resp, err := c.Get("https://fiddle.skia.org/reset_gpu/")
	if resp.StatusCode != 200 {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Failed to read response when resetting GPU: %s", err)
		} else {
			log.Fatalf("Failed to reset GPU: %s", string(b))
		}
	}

	// Spin up workers.
	for i := 0; i < *procs; i++ {
		g.Go(func() error {
			c := httputils.NewTimeoutClient()
			for req := range requestsCh {
				if !*quiet {
					fmt.Print(".")
				}
				if *force {
					req.req.Fast = false
				} else if lastWritten != nil {
					fiddleHash, err := req.req.Options.ComputeHash(req.req.Code)
					if err == nil {
						if lastWritten[req.id] != nil {
							if lastWritten[req.id].FiddleHash == fiddleHash {
								mutex.Lock()
								response[req.id] = lastWritten[req.id]
								mutex.Unlock()
								continue
							}
						}
					}
				}
				// POST to fiddle.
				b, err = json.Marshal(req.req)
				if err != nil {
					return fmt.Errorf("Failed to encode an individual request: %s", err)
				}
				resp, err := c.Post(*domain+"/_/run", "application/json", bytes.NewReader(b))
				if err != nil || resp.StatusCode != 200 {
					return fmt.Errorf("Failed to make request: %s", err)
				}

				// Decode response and add to all responses.
				runResults := types.RunResults{}
				if err := json.NewDecoder(resp.Body).Decode(&runResults); err != nil {
					return fmt.Errorf("Failed to read response: %s", err)
				}
				if resp.StatusCode != 200 {
					return fmt.Errorf("Request failed with %d: %s", resp.StatusCode, string(b))
				}
				mutex.Lock()
				response[req.id] = &runResults
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
