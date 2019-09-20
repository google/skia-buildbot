package main

import (
	"encoding/json"
	"os"

	"go.skia.org/infra/go/sklog"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
)

type StaticResources struct {
	Listeners []*envoy.Listener `json:"listeners"`
	Clusters  []*envoy.Cluster  `json:"clusters"`
}

type EnvoyStaticFile struct {
	StaticResources StaticResources `json:"static_resources"`
}

func main() {
	filenames := os.Args[1:]
	if len(filenames) != 2 {
		sklog.Fatalf("Usage: merge_envoy file1 file2")
	}
	// Read in file1
	var file1 EnvoyStaticFile
	f, err := os.Open(filenames[0])
	if err != nil {
		sklog.Fatal(err)
	}
	if err := json.NewDecoder(f).Decode(&file1); err != nil {
		sklog.Fatal(err)
	}

	// Read in file2
	var file2 EnvoyStaticFile
	f, err = os.Open(filenames[1])
	if err != nil {
		sklog.Fatal(err)
	}
	if err := json.NewDecoder(f).Decode(&file2); err != nil {
		sklog.Fatal(err)
	}

	// Merge Clusters.
	file2.StaticResources.Clusters = append(file2.StaticResources.Clusters, file1.StaticResources.Clusters...)
	//	file2.StaticResources.Listeners = append(file2.StaticResources.Listeners, file1.StaticResources.Listeners...)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(file2)
}
