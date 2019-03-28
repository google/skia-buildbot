package main

import (
	"fmt"
	"strconv"

	"go.skia.org/infra/go/human"
)

// gitSyncConfig contains the configuration options that can be defined in a config file.
// The JSON names of the fields match the flags defined in main.go.
type gitSyncConfig struct {
	BTInstanceID    string             `json:"bt_instance"` // BigTable instance
	BTTableID       string             `json:"bt_table"`    // BigTable table ID.
	HttpPort        string             `json:"http_port"`   // HTTP port for the health endpoint.
	Local           bool               `json:"local"`       // Indicating whether this is running local.
	ProjectID       string             `json:"project"`     // GCP project ID.
	PromPort        string             `json:"prom_port"`   // Port at which the Prometheus metrics are be exposed.
	RepoURLs        []string           `json:"repo_url"`    // List of repository URLs that should be updated.
	RefreshInterval human.JSONDuration `json:"refresh"`     // Interval at which to poll each git repository.
	WorkDir         string             `json:"workdir"`     // Work directory that should contain the checkouts.
}

// String returns all configuration settings as a string intended to be printed upon startup
// as feedback about the active configuration options.
func (g *gitSyncConfig) String() string {
	ret := ""
	prefix := "      "
	ret += fmt.Sprintf("%s bt_instance  : %s\n", prefix, g.BTInstanceID)
	ret += fmt.Sprintf("%s bt_table     : %s\n", prefix, g.BTTableID)
	ret += fmt.Sprintf("%s http_port    : %s\n", prefix, g.HttpPort)
	ret += fmt.Sprintf("%s local        : %s\n", prefix, strconv.FormatBool(g.Local))
	ret += fmt.Sprintf("%s project      : %s\n", prefix, g.ProjectID)
	ret += fmt.Sprintf("%s prom_port    : %s\n", prefix, g.PromPort)
	for _, url := range g.RepoURLs {
		ret += fmt.Sprintf("%s repo_url     : %s\n", prefix, url)
	}
	ret += fmt.Sprintf("%s refresh      : %s\n", prefix, g.RefreshInterval.String())
	ret += fmt.Sprintf("%s workdir      : %s\n", prefix, g.WorkDir)
	return ret
}
