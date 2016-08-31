// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/data/caching/proccache"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"golang.org/x/net/context"
)

// Options is the set of options used to set up a Manager.
//
// The configuration is loaded from a svcconfig.Config protobuf.
type Options struct {
	// Config is the configuration service to load from.
	Config config.Interface

	// ServiceID is used to load project configurations, which are named after a
	// specific service.
	//
	// If empty, project configurations cannot be loaded.
	ServiceID string

	// ConfigSet is the name of the ConfigSet to load.
	ConfigSet string
	// ServiceConfigPath is the name of the LogDog service config within the
	// ConfigSet.
	ServiceConfigPath string

	// ProjectConfigCacheDuration is the amount of time to cache a project's
	// configuration. If this is <= 0, the project config will be fetched each
	// time it's requested.
	ProjectConfigCacheDuration time.Duration

	// KillCheckInterval, if >0, starts a goroutine that polls every interval to
	// see if the configuration has changed. If it has, KillFunc will be invoked.
	KillCheckInterval time.Duration
	// KillFunc is the function that will be called if a configuration hash change
	// has been observed.
	KillFunc func()
}

func (o *Options) getConfig(c context.Context, hashOnly bool) (*config.Config, error) {
	return o.Config.GetConfig(c, o.ConfigSet, o.ServiceConfigPath, hashOnly)
}

func (o *Options) pollForConfigChanges(c context.Context, hash string) {
	for {
		log.Fields{
			"timeout": o.KillCheckInterval,
		}.Debugf(c, "Entering kill check poll loop...")

		if tr := clock.Sleep(c, o.KillCheckInterval); tr.Incomplete() {
			log.WithError(c.Err()).Debugf(c, "Context cancelled, shutting down kill poller.")
			return
		}

		log.Infof(c, "Kill check timeout triggered, checking configuration...")
		cfg, err := o.getConfig(c, true)
		if err != nil {
			log.WithError(err).Warningf(c, "Failed to reload configuration.")
			continue
		}

		if cfg.ContentHash != hash {
			log.Fields{
				"originalHash": hash,
				"newHash":      cfg.ContentHash,
			}.Errorf(c, "Configuration content hash has changed.")
			o.runKillFunc()
			return
		}

		log.Fields{
			"currentHash": cfg.ContentHash,
		}.Debugf(c, "Content hash matches.")
	}
}

func (o *Options) runKillFunc() {
	if f := o.KillFunc; f != nil {
		f()
	}
}

// Manager holds and exposes a service configuration.
//
// It can also periodically refresh that configuration and invoke a shutdown
// function if its content changes.
type Manager struct {
	o *Options

	// cfg is the initial configuration.
	cfg svcconfig.Config
	// cfgHash is the hash string of the original config.
	cfgHash string

	// projectConfigCache is a cache of project-specific configs. They will
	// eventually timeout and get refreshed according to options.
	projectConfigCache proccache.Cache

	// configChangedC will contain the result of the most recent configuration
	// change poll operation. The configuration change poller will block until
	// that result is consumed.
	configChangedC chan struct{}
	// changePollerCancelFunc is the cancel function to call to stop the
	// configuration poller.
	changePollerCancelFunc func()
}

// NewManager generates a new Manager and loads the initial configuration.
func NewManager(c context.Context, o Options) (*Manager, error) {
	m := Manager{
		o: &o,
	}

	// Load the initial configuration.
	if err := m.reloadConfig(c); err != nil {
		return nil, err
	}

	if o.KillCheckInterval > 0 {
		m.configChangedC = make(chan struct{})

		var cancelC context.Context
		cancelC, m.changePollerCancelFunc = context.WithCancel(c)
		go func() {
			defer close(m.configChangedC)
			m.o.pollForConfigChanges(cancelC, m.cfgHash)
		}()
	}

	return &m, nil
}

// Config returns the service configuration instance.
func (m *Manager) Config() *svcconfig.Config {
	return &m.cfg
}

// ProjectConfig returns the project configuration.
func (m *Manager) ProjectConfig(c context.Context, project config.ProjectName) (*svcconfig.ProjectConfig, error) {
	serviceID := m.o.ServiceID
	if serviceID == "" {
		return nil, errors.New("no service ID specified")
	}

	v, err := proccache.GetOrMake(c, project, func() (interface{}, time.Duration, error) {
		configSet := fmt.Sprintf("projects/%s", project)
		configPath := fmt.Sprintf("%s.cfg", serviceID)
		cfg, err := m.o.Config.GetConfig(c, configSet, configPath, false)
		if err != nil {
			log.Fields{
				log.ErrorKey: err,
				"project":    project,
				"configSet":  configSet,
				"path":       configPath,
			}.Errorf(c, "Failed to load config.")
			return nil, 0, err
		}

		var pcfg svcconfig.ProjectConfig
		if err := proto.UnmarshalText(cfg.Content, &pcfg); err != nil {
			log.Fields{
				log.ErrorKey: err,
				"project":    project,
				"configSet":  configSet,
				"path":       configPath,
				"hash":       cfg.ContentHash,
			}.Errorf(c, "Failed to unmarshal project config.")
		}

		log.Fields{
			"cacheDuration": m.o.ProjectConfigCacheDuration,
			"project":       project,
			"configSet":     configSet,
			"path":          configPath,
			"hash":          cfg.ContentHash,
		}.Infof(c, "Refreshed project configuration.")
		return &pcfg, m.o.ProjectConfigCacheDuration, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*svcconfig.ProjectConfig), nil
}

// Close terminates the config change poller and blocks until it has finished.
//
// Close must be called in order to ensure that Go scheduler properly schedules
// the goroutine.
func (m *Manager) Close() {
	// If our config change poller is running, cancel and reap it.
	if m.changePollerCancelFunc != nil {
		m.changePollerCancelFunc()
		<-m.configChangedC
	}
}

func (m *Manager) reloadConfig(c context.Context) error {
	cfg, err := m.o.getConfig(c, false)
	if err != nil {
		return err
	}

	if err := proto.UnmarshalText(cfg.Content, &m.cfg); err != nil {
		log.Fields{
			log.ErrorKey: err,
			"hash":       cfg.ContentHash,
		}.Errorf(c, "Failed to unmarshal configuration.")
		return err
	}
	m.cfgHash = cfg.ContentHash
	return nil
}
