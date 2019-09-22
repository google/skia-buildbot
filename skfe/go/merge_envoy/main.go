// merge_envoy takes two Envoy config files, adds all the Virtual Hosts and
// Clusters from the second file to the first file and then serializes the
// results of the merge as JSON to stdout.
package main

import (
	"fmt"
	"os"

	"github.com/Jeffail/gabs"
	"go.skia.org/infra/go/sklog"
)

func main() {
	filenames := os.Args[1:]
	if len(filenames) != 2 {
		sklog.Fatalf("Usage: merge_envoy file1 file2")
	}
	file1, err := gabs.ParseJSONFile(filenames[0])
	if err != nil {
		sklog.Fatal(err)
	}
	file2, err := gabs.ParseJSONFile(filenames[1])
	if err != nil {
		sklog.Fatal(err)
	}

	// Merge Clusters.
	clusterPath := []string{"static_resources", "clusters"}
	for _, cluster := range file2.S(clusterPath...).Children() {
		file1.ArrayAppend(cluster.Data(), clusterPath...)
	}

	// Merge Virtual Hosts.
	hostsPath := []string{"static_resources", "listeners", "0", "filter_chains", "filters", "0", "config", "route_config", "virtual_hosts"}
	for _, cluster := range file2.S(hostsPath...).Children() {
		file1.ArrayAppend(cluster.Data(), hostsPath...)
	}

	fmt.Print(file1.StringIndent("", "  "))
}
