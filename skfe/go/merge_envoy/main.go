// merge_envoy takes two Envoy config files, adds all the Virtual Hosts and
// Clusters from the second file to the first file and then serializes the
// results of the merge as JSON to stdout.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"go.skia.org/infra/go/sklog"
)

const tls = `{
	"name": "envoy.transport_sockets.tls",
	"typed_config": {
		"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext",
		"common_tls_context": {
			"tls_certificates": [
				{
					"certificate_chain": {
						"filename": "/etc/envoy/front-proxy-crt.pem"
					},
					"private_key": {
						"filename": "/etc/envoy/front-proxy-key.pem"
					}
				}
			]
		}
	}
 }`

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
	for _, cluster := range file2.Search(clusterPath...).Children() {
		if err := file1.ArrayAppend(cluster.Data(), clusterPath...); err != nil {
			sklog.Fatal(err)
		}
	}

	// Merge Virtual Hosts.
	hostsPath := []string{"static_resources", "listeners", "0", "filter_chains", "filters", "0", "typed_config", "route_config", "virtual_hosts"}
	for _, cluster := range file2.Search(hostsPath...).Children() {
		if err := file1.ArrayAppend(cluster.Data(), hostsPath...); err != nil {
			sklog.Fatal(err)
		}
	}

	// Duplicate the one listener so we can add it again, but at a different
	// port and with TLS.

	// Only do this for `skia-infra-public` for now, until we know this won't
	// break serving.
	if strings.HasSuffix(filenames[1], "skia-infra-public.json") {

		// Dulicate the first listener to create secondListener.
		firstListener := file1.Search("static_resources", "listeners", "0")
		copyAsString := firstListener.String()
		secondListener, err := gabs.ParseJSON([]byte(copyAsString))
		if err != nil {
			sklog.Fatal(err)
		}

		// Change port for secondListener.
		_, err = secondListener.Search("address", "socket_address").Set(8443, "port_value")
		if err != nil {
			sklog.Fatal(err)
		}

		// Add TLS config to secondListener.
		tlsConfig, err := gabs.ParseJSON([]byte(tls))
		if err != nil {
			sklog.Fatal(err)
		}
		_, err = secondListener.Set(tlsConfig, "filter_chains", "transport_socket")
		if err != nil {
			sklog.Fatal(err)
		}

		// Add secondListener back into the file.
		err = file1.ArrayAppend(secondListener, "static_resources", "listeners")
		if err != nil {
			sklog.Fatal(err)
		}
	}

	fmt.Print(file1.StringIndent("", "  "))
}
