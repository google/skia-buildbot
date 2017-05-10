// Command line app to process fiddles in bulk.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
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
	// Parse to validate it.
	request := &types.BulkRequest{}
	if err := json.Unmarshal(b, request); err != nil {
		log.Fatalf("%s does not contain valid JSON: %s", *input, err)
	}

	// POST to fiddle.
	c := httputils.NewTimeoutClient()
	resp, err := c.Post(*domain+"/_/bulk", "application/json", bytes.NewReader(b))
	if err != nil {
		log.Fatal("Failed to make request: %s", err)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to make request: %s", err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("Request failed with %d: %s", resp.StatusCode, string(b))
	}
	if err := ioutil.WriteFile(*output, b, 0600); err != nil {
		log.Fatalf("Failed to write response file: %s", err)
	}
}
