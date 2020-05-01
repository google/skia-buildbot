package main

import (
	"encoding/json"
	"flag"
	"os"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	jobsFile   = flag.String("jobs_file", "", "File that contains the job definitions.")
	outputFile = flag.String("out_file", "", "Output file where to write the expectations.")
)

func main() {
	common.Init()

	if (*jobsFile == "") || (*outputFile == "") {
		sklog.Fatalf("Jobs file and output file must be provided.")
	}

	// Load the jobs file and parse it.
	jobs := []string{}
	f, err := os.Open(*jobsFile)
	if err != nil {
		sklog.Fatalf("Unable to open jobs file: %s", err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&jobs); err != nil {
		sklog.Fatalf("Error parsing jobs file: %s", err)
	}

	// Split the builder names.
	models := util.StringSet{}
	for _, job := range jobs {
		if strings.HasPrefix(job, "Test") {
			parts := strings.Split(job, "-")
			if len(parts) > 3 {
				sklog.Infof("entry: %s", job)
				models[parts[3]] = true
			}
		}
	}

	// Write the output file.
	output := paramtools.ParamSet{"model": models.Keys()}
	if f, err = os.Create(*outputFile); err != nil {
		sklog.Fatalf("Error creating output file: %s", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(output); err != nil {
		sklog.Fatalf("Error encoding output file: %s", err)
	}
}
