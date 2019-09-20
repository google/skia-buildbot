package main

import (
	"encoding/json"
	"os"

	"go.skia.org/infra/go/sklog"
)

type RouteConfig struct {
	VirtualHosts []interface{} `json:"virtual_hosts"`
}

type Config struct {
	StatPrefix  string      `json:"stat_prefix"`
	HTTPFilters interface{} `json:"http_filters"`
	RouteConfig RouteConfig `json:"route_config"`
}

type Filter struct {
	Name   string `json:"name"`
	Config Config `json:"config"`
}

type FilterChains struct {
	Filters []*Filter `json:"filters"`
}

type Listener struct {
	Address      interface{}  `json:"address"`
	FilterChains FilterChains `json:"filter_chains"`
}
type StaticResources struct {
	Listeners []*Listener   `json:"listeners"`
	Clusters  []interface{} `json:"clusters"`
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
	file1.StaticResources.Clusters = append(file1.StaticResources.Clusters, file2.StaticResources.Clusters...)

	// Merge Virtual Hosts.
	file1.StaticResources.Listeners[0].FilterChains.Filters[0].Config.RouteConfig.VirtualHosts = append(
		file1.StaticResources.Listeners[0].FilterChains.Filters[0].Config.RouteConfig.VirtualHosts,
		file2.StaticResources.Listeners[0].FilterChains.Filters[0].Config.RouteConfig.VirtualHosts...)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(file1)
}
