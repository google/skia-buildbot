// Package clusterconfig contains helper functions for dealing with the
// configuration of all Skia Infra k8s clusters. See /infra/kube/README.md for
// an overview of how clusters are managed.
package clusterconfig

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/kube/clusters"
)

// Cluster is detailed info on a particular cluster in a ClusterConfig.
type Cluster struct {
	// Type is the type of cluster: always "gke", previously could be "k3s".
	Type string `json:"type"`

	// Zone is the GCE zone.
	Zone string `json:"zone"`

	// Project is the GCE project name, e.g. google.com:skia-corp.
	Project string `json:"project"`

	// ContextName is the name of the context the cluster should have e.g. gke_skia-public_us-central1-a_skia-public
	ContextName string `json:"context_name"`
}

// ClusterConfig describes the format of the infra/kube/clusters/config.json file.
type ClusterConfig struct {
	// GitDir is where to check out Repo.
	GitDir string `json:"gitdir"`

	// Repo is the clone URL of the Git repo that contains our YAML files.
	Repo string `json:"repo"`

	// Clusters maps the common kubernetes cluster name to the details for that cluster.
	Clusters map[string]Cluster `json:"clusters"`
}

// New returns a ClusterConfig for accessing the config.json file that contains
// information on each cluster we use.
//
// If configFile is the empty string then the config.json file relative to this
// source file will be loaded.
//
// See /infra/kube/README.md for a description of the config.json file format.
func New(configFile string) (ClusterConfig, error) {
	var ret ClusterConfig

	// Set the config path, start with flag, fall back to relative location in
	// the source tree.
	configFilename := configFile
	if configFilename == "" {
		_, filename, _, _ := runtime.Caller(0)
		configFilename = filepath.Join(filepath.Dir(filename), "../../../kube/clusters/config.json")
	}

	b, err := ioutil.ReadFile(configFilename)
	if err != nil {
		return ret, skerr.Wrap(err)
	}
	if err := json.Unmarshal(b, &ret); err != nil {
		return ret, skerr.Wrap(err)
	}
	if overrideDir := os.Getenv("PUSHK_GITDIR"); overrideDir != "" {
		ret.GitDir = overrideDir
	}
	return ret, nil
}

// NewFromEmbeddedConfig returns a new ClusterConfig from the embedded
// config.json file in //kube/clusters.
func NewFromEmbeddedConfig() (*ClusterConfig, error) {
	var ret ClusterConfig
	if err := json.Unmarshal([]byte(clusters.ClusterConfig), &ret); err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode embedded cluster config.")
	}
	return &ret, nil
}

// NewWithCheckout returns a ClusterConfig for accessing the config.json file
// that contains information on each cluster we use, and also checks out the
// YAML files for all the clusters.
//
// If configFile is the empty string then the config.json file relative to this
// source file will be loaded.
//
// See /infra/kube/README.md for a description of the config.json file format.
func NewWithCheckout(ctx context.Context, configFile string) (ClusterConfig, *git.Checkout, error) {
	cfg, err := New(configFile)
	if err != nil {
		return cfg, nil, skerr.Wrap(err)
	}
	checkout, err := git.NewCheckout(ctx, cfg.Repo, cfg.GitDir)
	if err != nil {
		return cfg, nil, skerr.Wrap(err)
	}
	return cfg, checkout, nil
}
