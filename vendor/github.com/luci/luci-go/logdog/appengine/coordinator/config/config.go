// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package config

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"golang.org/x/net/context"
)

var (
	// ErrInvalidConfig is returned when the configuration exists, but is invalid.
	ErrInvalidConfig = errors.New("invalid configuration")
)

// Config is the LogDog Coordinator service configuration.
type Config struct {
	svcconfig.Config
	// Settings are per-instance settings.
	Settings Settings

	// ConfigServiceURL is the config service's URL.
	ConfigServiceURL url.URL `json:"-"`
	// ConfigSet is the name of the service config set that is being used.
	ConfigSet string `json:"-"`
	// ServiceConfigPath is the path within ConfigSet of the service
	// configuration.
	ServiceConfigPath string `json:"-"`
}

// ServiceConfigPath returns the config set and path for this application's
// service configuration.
func ServiceConfigPath(c context.Context) (string, string) {
	appID := info.Get(c).AppID()
	return fmt.Sprintf("services/%s", appID), svcconfig.ServiceConfigFilename
}

// Load loads the service configuration. This includes:
//	- The config service settings.
//	- The service configuration, loaded from the config service.
//	- Additional Settings data from datastore via settings.
//
// The service config is minimally validated prior to being returned.
func Load(c context.Context) (*Config, error) {
	configSet, configPath := ServiceConfigPath(c)
	serviceCfg, err := config.GetConfig(c, configSet, configPath, false)
	if err != nil {
		log.Fields{
			log.ErrorKey: err,
			"configSet":  configSet,
			"configPath": configPath,
		}.Errorf(c, "Failed to load configuration from config service.")
		return nil, err
	}

	// Unmarshal the config into service configuration.
	cfg := Config{
		ConfigServiceURL:  config.ServiceURL(c),
		ConfigSet:         serviceCfg.ConfigSet,
		ServiceConfigPath: serviceCfg.Path,
	}

	if err := proto.UnmarshalText(serviceCfg.Content, &cfg.Config); err != nil {
		log.Fields{
			log.ErrorKey:  err,
			"size":        len(serviceCfg.Content),
			"contentHash": serviceCfg.ContentHash,
			"configSet":   serviceCfg.ConfigSet,
			"revision":    serviceCfg.Revision,
		}.Errorf(c, "Failed to unmarshal configuration protobuf.")
		return nil, ErrInvalidConfig
	}

	// Validate the configuration.
	if err := validateServiceConfig(&cfg.Config); err != nil {
		log.WithError(err).Errorf(c, "Invalid Coordinator configuration.")
		return nil, ErrInvalidConfig
	}

	// Load our settings.
	if err := cfg.Settings.Load(c); err != nil {
		log.WithError(err).Errorf(c, "Failed to load settings.")
		return nil, ErrInvalidConfig
	}

	return &cfg, nil
}

// validateServiceConfig checks the supplied service config object to ensure
// that it meets a minimum configuration standard expected by our endpoitns and
// handlers.
func validateServiceConfig(cc *svcconfig.Config) error {
	switch {
	case cc == nil:
		return errors.New("configuration is nil")
	case cc.GetCoordinator() == nil:
		return errors.New("no Coordinator configuration")
	default:
		return nil
	}
}
