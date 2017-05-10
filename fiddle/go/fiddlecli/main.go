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

	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/httputils"
)

// flags
var (
	domain = flag.String("domain", "https://fiddle.skia.org", "Where to send the JSON request.")
	input  = flag.String("input", "", "The name of the file to read the JSON from.")
	output = flag.String("output", "", "The name of the file to write the JSON results to.")
	quiet  = flag.Bool("quiet", false, "Run without a progress bar.")
	force  = flag.Bool("force", false, "Force a compile and run for each fiddle, don't take the fast path.")
)

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

	// Loop over each entry and run it.
	c := httputils.NewTimeoutClient()
	response := types.BulkResponse{}
	for id, req := range requests {
		if !*quiet {
			fmt.Print(".")
		}
		if *force {
			req.Fast = false
		}
		// POST to fiddle.
		b, err = json.Marshal(req)
		if err != nil {
			log.Fatalf("Failed to encode an individual request: %s", err)
		}
		resp, err := c.Post(*domain+"/_/run", "application/json", bytes.NewReader(b))
		if err != nil {
			log.Fatalf("Failed to make request: %s", err)
		}

		// Decode response and add to all responses.
		runResults := types.RunResults{}
		if err := json.NewDecoder(resp.Body).Decode(&runResults); err != nil {
			log.Fatalf("Failed to read response: %s", err)
		}
		if resp.StatusCode != 200 {
			log.Fatalf("Request failed with %d: %s", resp.StatusCode, string(b))
		}
		response[id] = &runResults
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
