// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package config

import (
	"fmt"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"golang.org/x/net/context"
)

const maxProjectWorkers = 32

// ProjectConfigPath returns the path of the project-specific configuration.
// This path should be used with a project config set.
//
// A given project's configuration is named after the current App ID.
func ProjectConfigPath(c context.Context) string {
	return fmt.Sprintf("%s.cfg", info.Get(c).AppID())
}

// ProjectConfig loads the project config protobuf from the config service.
//
// If the configuration was not present, config.ErrNoConfig will be returned.
func ProjectConfig(c context.Context, project config.ProjectName) (*svcconfig.ProjectConfig, error) {
	if project == "" {
		return nil, config.ErrNoConfig
	}

	configSet, configPath := config.ProjectConfigSet(project), ProjectConfigPath(c)
	cfg, err := config.GetConfig(c, configSet, configPath, false)
	if err != nil {
		return nil, err
	}

	var pcfg svcconfig.ProjectConfig
	if err := proto.UnmarshalText(cfg.Content, &pcfg); err != nil {
		log.Fields{
			log.ErrorKey:  err,
			"configSet":   cfg.ConfigSet,
			"path":        cfg.Path,
			"contentHash": cfg.ContentHash,
		}.Errorf(c, "Failed to unmarshal project configuration.")
		return nil, ErrInvalidConfig
	}

	return &pcfg, nil
}

// AllProjectConfigs returns the project configurations for all projects that
// have a configuration.
//
// If a project's configuration fails to load, an error will be logged and the
// project will be omitted from the output map.
func AllProjectConfigs(c context.Context) (map[config.ProjectName]*svcconfig.ProjectConfig, error) {
	// TODO: This endpoint is generally slow. Even though there is memcache-based
	// config cache, this really should be loaded from a more failsafe cache like
	// datastore to protect against config service outages.
	configs, err := config.GetProjectConfigs(c, ProjectConfigPath(c), false)
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to load project configs.")
		return nil, err
	}

	result := make(map[config.ProjectName]*svcconfig.ProjectConfig, len(configs))
	for _, cfg := range configs {
		// Identify the project by removng the "projects/" prefix.
		project := config.ProjectName(strings.TrimPrefix(cfg.ConfigSet, "projects/"))
		if err := project.Validate(); err != nil {
			log.Fields{
				log.ErrorKey: err,
				"configSet":  cfg.ConfigSet,
			}.Errorf(c, "Invalid project name returned.")
			continue
		}

		// Unmarshal the project's configuration.
		pcfg, err := loadProjectConfig(&cfg)
		if err != nil {
			log.Fields{
				log.ErrorKey:  err,
				"configSet":   cfg.ConfigSet,
				"path":        cfg.Path,
				"contentHash": cfg.ContentHash,
			}.Errorf(c, "Failed to unmarshal project configuration.")
			continue
		}

		result[project] = pcfg
	}
	return result, nil
}

func loadProjectConfig(cfg *config.Config) (*svcconfig.ProjectConfig, error) {
	var pcfg svcconfig.ProjectConfig
	if err := proto.UnmarshalText(cfg.Content, &pcfg); err != nil {
		return nil, err
	}
	return &pcfg, nil
}
