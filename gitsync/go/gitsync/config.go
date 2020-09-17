package main

import (
	"fmt"
	"strconv"

	"go.skia.org/infra/go/human"
)

// gitSyncConfig contains the configuration options that can be defined in a config file.
// The JSON names of the fields match the flags defined in main.go.
type gitSyncConfig struct {
	// BigTable instance.
	BTInstanceID string `json:"bt_instance"`
	// BigTable table ID.
	BTTableID string `json:"bt_table"`
	// Number of goroutines to use when writing to BigTable. This is a
	// tradeoff between write throughput and memory usage; more goroutines
	// will achieve higher throughput but will also use more memory. There
	// are diminishing returns here, as the number of CPU cores and BigTable
	// performance will also limit throughput. The default value in
	// bt_gitstore.DefaultWriteGoroutines has been shown to keep memory
	// usage within a reasonable range while still providing decent
	// throughput; you should only need to override this value in the case
	// of high memory pressure (fewer goroutines) or the initial ingestion
	// of an exceptionally large repository (more goroutines).
	BTWriteGoroutines int `json:"bt_write_goroutines"`
	// HTTP port for the health endpoint.
	HttpPort string `json:"http_port"`
	// IncludeBranches specifies which branches of the given repo should be
	// included; any others are ignored. These are given in the format:
	// <repo URL>=<branch_name>[,<branch_name>]*
	IncludeBranches []string `json:"include_branches"`
	// Indicating whether this is running local.
	Local bool `json:"local"`
	// Mirrors indicate that the data obtained for a given repo should come
	// from a Gitiles mirror at a different location. These are given in the
	// format: <repo URL>=<gitiles mirror URL>
	Mirrors []string `json:"mirrors"`
	// GCP project ID.
	ProjectID string `json:"project"`
	// Port at which the Prometheus metrics are be exposed.
	PromPort string `json:"prom_port"`
	// List of repository URLs that should be updated.
	RepoURLs []string `json:"repo_url"`
	// Interval at which to poll each git repository.
	RefreshInterval human.JSONDuration `json:"refresh"`
	// Work directory that should contain the checkouts.
	WorkDir string `json:"workdir"`
}

// String returns all configuration settings as a string intended to be printed upon startup
// as feedback about the active configuration options.
func (g *gitSyncConfig) String() string {
	ret := ""
	prefix := "      "
	ret += fmt.Sprintf("%s bt_instance        : %s\n", prefix, g.BTInstanceID)
	ret += fmt.Sprintf("%s bt_table           : %s\n", prefix, g.BTTableID)
	ret += fmt.Sprintf("%s bt_write_goroutines: %d\n", prefix, g.BTWriteGoroutines)
	ret += fmt.Sprintf("%s http_port          : %s\n", prefix, g.HttpPort)
	ret += fmt.Sprintf("%s local              : %s\n", prefix, strconv.FormatBool(g.Local))
	for _, mirror := range g.Mirrors {
		ret += fmt.Sprintf("%s mirror             : %s\n", prefix, mirror)
	}
	ret += fmt.Sprintf("%s project            : %s\n", prefix, g.ProjectID)
	ret += fmt.Sprintf("%s prom_port          : %s\n", prefix, g.PromPort)
	for _, url := range g.RepoURLs {
		ret += fmt.Sprintf("%s repo_url           : %s\n", prefix, url)
	}
	ret += fmt.Sprintf("%s refresh            : %s\n", prefix, g.RefreshInterval.String())
	ret += fmt.Sprintf("%s workdir            : %s\n", prefix, g.WorkDir)
	return ret
}
