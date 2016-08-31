// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"path/filepath"

	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/deploytool/api/deploy"

	"github.com/mitchellh/go-homedir"
	"golang.org/x/net/context"
)

const userCfgName = ".luci_deploy.cfg"

func loadUserConfig(c context.Context, cfg *deploy.UserConfig) error {
	path, err := homedir.Dir()
	if err != nil {
		log.WithError(err).Warningf(c, "Failed to get user home directory; cannot load local config.")
		return nil
	}

	configPath := filepath.Join(path, userCfgName)
	switch err := unmarshalTextProtobuf(configPath, cfg); {
	case err == nil:
		log.Infof(c, "Loaded user config from: %s", configPath)

	case isNotExist(err):
		log.Debugf(c, "No user config found at: %s", configPath)

	default:
		return errors.Annotate(err).Reason("failed to load config at [%(path)s]").D("path", configPath).Err()
	}

	return nil
}
