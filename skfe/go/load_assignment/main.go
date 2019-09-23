// load_assignment takes an Envoy config file, and converts all the Clusters
// hosts to load_assignment, because hosts is deprecated.
package main

import (
	"fmt"
	"os"

	"github.com/Jeffail/gabs/v2"
	"go.skia.org/infra/go/sklog"
)

func main() {
	filenames := os.Args[1:]
	if len(filenames) != 1 {
		sklog.Fatalf("Usage: load_assignment file1")
	}
	file1, err := gabs.ParseJSONFile(filenames[0])
	if err != nil {
		sklog.Fatal(err)
	}

	// Merge Clusters.
	clusterPath := []string{"static_resources", "clusters"}
	for _, cluster := range file1.S(clusterPath...).Children() {
		// Read the values out of hosts.
		address := cluster.S("hosts", "0", "socket_address", "address").Data().(string)
		portValue := cluster.S("hosts", "0", "socket_address", "port_value").Data().(float64)

		// Write into load_assignment.
		cluster.Set(address, "load_assignment", "cluster_name")
		cluster.ArrayOfSize(1, "load_assignment", "endpoints")
		cluster.Object("load_assignment", "endpoints", "0")
		cluster.ArrayOfSize(1, "load_assignment", "endpoints", "0", "lb_endpoints")
		cluster.Object("load_assignment", "endpoints", "0", "lb_endpoints", "0")
		cluster.Set(address, "load_assignment", "endpoints", "0", "lb_endpoints", "0", "endpoint", "address", "socket_address", "address")
		cluster.Set(portValue, "load_assignment", "endpoints", "0", "lb_endpoints", "0", "endpoint", "address", "socket_address", "port_value")

		// Remove hosts.
		cluster.Delete("hosts")
	}

	fmt.Print(file1.StringIndent("", "  "))
}
