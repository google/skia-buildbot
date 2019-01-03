package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"go.skia.org/infra/go/exec"
)

var (
	project      = flag.String("project", "", "[REQUIRED] The GCP project to analyze usage for.")
	displayBytes = flag.Bool("display_bytes", false, "Show the results in bytes, not in human readable form.")
)

func main() {
	flag.Parse()
	if *project == "" {
		fmt.Println("--project is required")
		flag.PrintDefaults()
		os.Exit(1)
	}

	fmt.Println("Fetching buckets in project")
	output := bytes.Buffer{}
	err := exec.Run(context.Background(), &exec.Command{
		Name:   "gsutil",
		Args:   []string{"ls", "-p", *project},
		Stdout: &output,
	})
	if err != nil {
		fmt.Printf("Could not retrieve buckets: %s\n", err)
		os.Exit(1)
	}

	buckets := strings.Split(strings.TrimSpace(output.String()), "\n")
	fmt.Printf("Found %d buckets\n", len(buckets))
	fmt.Println("Tabulating total space, this may take tens of seconds for big buckets")

	flags := "-hs"
	if *displayBytes {
		flags = "-s"
	}

	// Do them one at a time to show incremental progress, as large
	// buckets can take >10s to tabulate.
	for _, b := range buckets {
		output := bytes.Buffer{}
		// GCS buckets must be uniquely named, so no need to specify a project.
		err := exec.Run(context.Background(), &exec.Command{
			Name:   "gsutil",
			Args:   []string{"du", flags, b},
			Stdout: &output,
		})
		if err != nil {
			fmt.Printf("Could not get size for bucket %s: %s\n", b, err)
			continue
		}
		fmt.Println(strings.TrimSpace(output.String()))
	}
	fmt.Println("Done")
}
