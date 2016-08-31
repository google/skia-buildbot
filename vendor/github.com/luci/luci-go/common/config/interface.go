// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package config

import (
	"errors"
	"net/url"

	"golang.org/x/net/context"
)

// ErrNoConfig is returned if requested config does not exist.
var ErrNoConfig = errors.New("no such config")

// Config is a configuration entry in the luci-config service.
type Config struct {
	// ConfigSet is the config set name (e.g. "projects/<id>") this config
	// belongs to.
	// May be the empty string if this is unknown.
	ConfigSet string `json:"configSet,omitempty"`

	// Path is the filename relative to the root of the config set,
	// without leading slash, e.g. "luci-cron.cfg".
	// May be the empty string if this is unknown.
	Path string

	// Error is not nil if there where troubles fetching this config. Used only
	// by functions that operate with multiple configs at once, such as
	// GetProjectConfigs and GetRefConfigs.
	Error error `json:"error,omitempty"`

	// Content is the actual body of the config file.
	Content string `json:"content,omitempty"`

	// ContentHash can be used to quickly check that content didn't change.
	ContentHash string `json:"contentHash,omitempty"`

	// Revision is git SHA1 of a repository the config was fetched from.
	Revision string `json:"revision,omitempty"`
}

// RepoType is the type of the repo the Project is stored in.
type RepoType string

const (
	// GitilesRepo means a repo is backed by the Gitiles service.
	GitilesRepo RepoType = "GITILES"

	// UnknownRepo means a repo is backed by an unknown service.
	// It may be an invalid repo.
	UnknownRepo RepoType = "UNKNOWN"
)

// Project is a project registered in the luci-config service.
type Project struct {
	// ID is unique project identifier.
	ID string

	// Name is a short friendly display name of the project.
	Name string

	// RepoType specifies in what kind of storage projects configs are stored.
	RepoType RepoType

	// RepoUrl is the location of this project code repository. May be nil if
	// unknown or cannot be parsed.
	RepoURL *url.URL
}

// Interface represents luci-config service API.
//
// All methods accept context.Context they use for deadlines and for passing to
// callbacks (if the implementation uses any). Contexts here don't necessary
// relate to a "global" package context (used by GetImplementation and
// SetImplementation), though very often they are the same (as is the case when
// using package-level functions like GetConfig).
//
// For example, unit tests may instantiate an implementation of Interface
// directly and don't bother registering it in the context with
// SetImplementation(...).
//
// Transient errors are wrapped in errors.Transient. See common/errors.
type Interface interface {
	// ServiceURL returns the URL of the config service.
	ServiceURL(ctx context.Context) url.URL

	// GetConfig returns a config at a path in a config set or ErrNoConfig
	// if missing. If hashOnly is true, returned Config struct has Content set
	// to "" (and the call is faster).
	GetConfig(ctx context.Context, configSet, path string, hashOnly bool) (*Config, error)

	// GetConfigByHash returns the contents of a config, as identified by its
	// content hash, or ErrNoConfig if missing.
	GetConfigByHash(ctx context.Context, contentHash string) (string, error)

	// GetConfigSetLocation returns the URL location of a config set.
	GetConfigSetLocation(ctx context.Context, configSet string) (*url.URL, error)

	// GetProjectConfigs returns all the configs at the given path in all
	// projects that have such config. If hashesOnly is true, returned Config
	// structs have Content set to "" (and the call is faster).
	GetProjectConfigs(ctx context.Context, path string, hashesOnly bool) ([]Config, error)

	// GetProjects returns all the registered projects in the configuration
	// service.
	GetProjects(ctx context.Context) ([]Project, error)

	// GetRefConfigs returns the config at the given path in all refs of all
	// projects that have such config. If hashesOnly is true, returned Config
	// structs have Content set to "" (and the call is faster).
	GetRefConfigs(ctx context.Context, path string, hashesOnly bool) ([]Config, error)

	// GetRefs returns the list of refs for a project.
	GetRefs(ctx context.Context, projectID string) ([]string, error)
}
