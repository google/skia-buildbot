// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"strings"

	"github.com/luci/gae/service/info"
	luciConfig "github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"github.com/luci/luci-go/logdog/appengine/coordinator/config"
	"golang.org/x/net/context"
)

const (
	// projectNamespacePrefix is the datastore namespace prefix for project
	// namespaces.
	projectNamespacePrefix = "luci."
)

// ProjectNamespace returns the AppEngine namespace for a given luci-config
// project name.
func ProjectNamespace(project luciConfig.ProjectName) string {
	return projectNamespacePrefix + string(project)
}

// ProjectFromNamespace returns the current project installed in the supplied
// Context's namespace.
//
// If the namespace does not have a project namespace prefix, this function
// will return an empty string.
func ProjectFromNamespace(ns string) luciConfig.ProjectName {
	if !strings.HasPrefix(ns, projectNamespacePrefix) {
		return ""
	}
	return luciConfig.ProjectName(ns[len(projectNamespacePrefix):])
}

// CurrentProject returns the current project based on the currently-loaded
// namespace.
//
// If there is no current namespace, or if the current namespace is not a valid
// project namespace, an empty string will be returned.
func CurrentProject(c context.Context) luciConfig.ProjectName {
	if ns, ok := info.Get(c).GetNamespace(); ok {
		return ProjectFromNamespace(ns)
	}
	return ""
}

// CurrentProjectConfig returns the project-specific configuration for the
// current project.
//
// If there is no current project namespace, or if the current project has no
// configuration, config.ErrInvalidConfig will be returned.
func CurrentProjectConfig(c context.Context) (*svcconfig.ProjectConfig, error) {
	return GetServices(c).ProjectConfig(c, CurrentProject(c))
}

// ActiveUserProjects returns a full list of all config service projects with
// LogDog project configurations that the current user has READ access to.
//
// TODO: Load project configs and all project configs lists from datastore. Add
// a background cron job to periodically update these lists from luci-config.
// This should be a generic config service capability.
func ActiveUserProjects(c context.Context) (map[luciConfig.ProjectName]*svcconfig.ProjectConfig, error) {
	allPcfgs, err := config.AllProjectConfigs(c)
	if err != nil {
		return nil, err
	}

	for project, pcfg := range allPcfgs {
		// Verify user READ access.
		if err := IsProjectReader(c, pcfg); err != nil {
			delete(allPcfgs, project)

			// If it is a membership error, prune this project and continue.
			// Otherwise, forward the error.
			if !IsMembershipError(err) {
				// No configuration for this project, the configuration is invalid, or
				// the user didn't have access. Remove it from the list.
				log.Fields{
					log.ErrorKey: err,
					"project":    project,
				}.Errorf(c, "Failed to check project.")
				return nil, err
			}
		}
	}
	return allPcfgs, nil
}
