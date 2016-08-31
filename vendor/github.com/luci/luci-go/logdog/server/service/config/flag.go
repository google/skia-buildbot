// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package config

import (
	"errors"
	"flag"
	"net/http"
	"time"

	"github.com/luci/luci-go/common/clock/clockflag"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/config/impl/filesystem"
	"github.com/luci/luci-go/common/config/impl/remote"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/services/v1"
	"golang.org/x/net/context"
)

// Flags is the set of command-line flags used to configure a service.
type Flags struct {
	// ConfigFilePath is the path within the ConfigSet of the configuration.
	//
	// If ConfigURL is empty, this will be interpreted as a local filesystem path
	// from which the configuration should be loaded.
	ConfigFilePath string

	// KillCheckInterval, if >0, starts a goroutine that polls every interval to
	// see if the configuration has changed.
	KillCheckInterval clockflag.Duration

	// RoundTripper, if not nil, is the http.RoundTripper that will be used to
	// fetch remote configurations.
	RoundTripper http.RoundTripper
}

// AddToFlagSet augments the supplied FlagSet with values for Flags.
func (f *Flags) AddToFlagSet(fs *flag.FlagSet) {
	fs.StringVar(&f.ConfigFilePath, "config-file-path", "",
		"If set, load configuration from a local filesystem rooted here.")
	fs.Var(&f.KillCheckInterval, "config-kill-interval",
		"If non-zero, poll for configuration changes and kill the application if one is detected.")
}

// CoordinatorOptions returns an Options instance loaded from the supplied flags
// and Coordinator instance.
func (f *Flags) CoordinatorOptions(c context.Context, client logdog.ServicesClient) (*Options, error) {
	ccfg, err := client.GetConfig(c, &google.Empty{})
	if err != nil {
		return nil, err
	}

	// If a ConfigFilePath was specified, use a mock configuration service that
	// loads from a local file.
	var ci config.Interface
	if f.ConfigFilePath != "" {
		var err error
		ci, err = filesystem.New(f.ConfigFilePath)
		if err != nil {
			return nil, err
		}
	} else {
		if ccfg.ConfigServiceUrl == "" {
			return nil, errors.New("coordinator does not specify a config service")
		}
		if ccfg.ConfigSet == "" {
			return nil, errors.New("coordinator does not specify a config set")
		}
		if ccfg.ServiceConfigPath == "" {
			return nil, errors.New("coordinator does not specify a config path")
		}

		var clientFactory remote.ClientFactory
		if f.RoundTripper != nil {
			rr := f.RoundTripper
			clientFactory = func(context.Context) (*http.Client, error) {
				return &http.Client{Transport: rr}, nil
			}
		}
		ci = remote.New(ccfg.ConfigServiceUrl, clientFactory)
	}

	return &Options{
		Config:            ci,
		ConfigSet:         ccfg.ConfigSet,
		ServiceConfigPath: ccfg.ServiceConfigPath,
		KillCheckInterval: time.Duration(f.KillCheckInterval),
	}, nil
}
