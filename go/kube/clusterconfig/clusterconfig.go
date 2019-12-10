// Package clusterconfig contains helper functions for dealing with the
// configuration of all Skia Infra k8s clusters. See /infra/kube/README.md for
// an overview of how clusters are managed.
package clusterconfig

import (
	"context"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// New returns a *viper.Viper for accessing the config.json file that contains
// information on each cluster we use.
//
// If configFile is the empty string then the config.json file relative to this
// source file will be loaded.
//
// See /infra/kube/README.md for a description of the config.json file format.
func New(configFile string) (*viper.Viper, error) {
	v := viper.New()
	v.SetEnvPrefix("pushk") // will be uppercased automatically

	// PUSHK_GITDIR will override "gitdir" in the config file.
	if err := v.BindEnv("gitdir"); err != nil {
		return nil, skerr.Wrap(err)
	}

	// Set the config path, start with flag, fall back to relative location in
	// the source tree.
	configFilename := configFile
	if configFilename == "" {
		_, filename, _, _ := runtime.Caller(0)
		configFilename = filepath.Join(filepath.Dir(filename), "../../../kube/clusters/config.json")
	}
	v.SetConfigFile(configFilename)

	err := v.ReadInConfig()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return v, nil
}

// NewWithCheckout returns a *viper.Viper for accessing the config.json file
// that contains information on each cluster we use, and also checks out the
// YAML files for all the clusters.
//
// If configFile is the empty string then the config.json file relative to this
// source file will be loaded.
//
// See /infra/kube/README.md for a description of the config.json file format.
func NewWithCheckout(ctx context.Context, configFile string) (*viper.Viper, *git.Checkout, error) {
	v, err := New(configFile)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	checkout, err := git.NewCheckout(ctx, v.GetString("repo"), v.GetString("gitdir"))
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	return v, checkout, nil
}
