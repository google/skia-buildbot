// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distributor

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/dm/api/distributor"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
)

var regKey = "holds a DM Distributor Registry"

// WithRegistry adds the registry to the Context.
func WithRegistry(c context.Context, r Registry) context.Context {
	if r == nil {
		panic(errors.New("you may not use WithRegistry on a nil Registry"))
	}
	return context.WithValue(c, &regKey, r)
}

// GetRegistry gets the registry from the Context. This will return nil if the
// Context does not contain a Registry.
func GetRegistry(c context.Context) Registry {
	ret, _ := c.Value(&regKey).(Registry)
	return ret
}

// FinishExecutionFn is required to eliminate a circular dependency
// between mutate <-> distributor. Essentially this just makes a new
// mutate.FinishExecution.
//
// See mutate.FinishExecutionFn for the only actual implementation of this.
type FinishExecutionFn func(c context.Context, eid *dm.Execution_ID, rslt *dm.Result) ([]tumble.Mutation, error)

// Registry holds a collection of all of the available distributor types.
type Registry interface {
	FinishExecution(c context.Context, eid *dm.Execution_ID, rslt *dm.Result) ([]tumble.Mutation, error)

	// MakeDistributor builds a distributor instance that's configured with the
	// provided config.
	//
	// The configuration for this distributor are obtained from luci-config at the
	// time an Execution is started.
	MakeDistributor(c context.Context, cfgName string) (d D, ver string, err error)
}

// FactoryMap maps nil proto.Message instances (e.g. (*MyMessage)(nil)) to the
// factory which knows how to turn a Message of that type into a distributor.
type FactoryMap map[proto.Message]Factory

// NewRegistry builds a new implementation of Registry configured to load
// configuration data from luci-config.
//
// The mapping should hold nil-ptrs of various config protos -> respective
// Factory. When loading from luci-config, when we see a given message type,
// we'll construct the distributor instance using the provided Factory.
func NewRegistry(mapping FactoryMap, fFn FinishExecutionFn) Registry {
	ret := &registry{fFn, make(map[reflect.Type]Factory, len(mapping))}
	add := func(p proto.Message, factory Factory) {
		if factory == nil {
			panic("factory is nil")
		}
		if p == nil {
			panic("proto.Message is nil")
		}

		typ := reflect.TypeOf(p)

		if _, ok := ret.data[typ]; ok {
			panic(fmt.Errorf("trying to register %q twice", typ))
		}
		ret.data[typ] = factory
	}
	for p, f := range mapping {
		add(p, f)
	}
	return ret
}

type registry struct {
	finishExecutionImpl FinishExecutionFn
	data                map[reflect.Type]Factory
}

var _ Registry = (*registry)(nil)

func (r *registry) FinishExecution(c context.Context, eid *dm.Execution_ID, rslt *dm.Result) ([]tumble.Mutation, error) {
	return r.finishExecutionImpl(c, eid, rslt)
}

func (r *registry) MakeDistributor(c context.Context, cfgName string) (d D, ver string, err error) {
	cfg, err := loadConfig(c, cfgName)
	if err != nil {
		logging.Fields{"error": err, "cfg": cfgName}.Errorf(c, "Failed to load config")
		return
	}

	ver = cfg.Version

	typ := reflect.TypeOf(cfg.Content)

	fn, ok := r.data[typ]
	if !ok {
		return nil, "", fmt.Errorf("unknown distributor type %T", cfg.Content)
	}

	d, err = fn(c, cfg)
	return
}

// loadConfig loads the named distributor configuration from luci-config,
// possibly using the in-memory or memcache version.
func loadConfig(c context.Context, cfgName string) (ret *Config, err error) {
	aid := info.Get(c).TrimmedAppID()
	distCfgObj, err := config.GetConfig(c, fmt.Sprintf("services/%s", aid), "distributors.cfg", false)
	if err != nil {
		return
	}

	cfgVersion := distCfgObj.Revision
	distCfg := &distributor.Config{}
	if err = proto.UnmarshalText(distCfgObj.Content, distCfg); err != nil {
		return
	}

	cfg, ok := distCfg.DistributorConfigs[cfgName]
	if !ok {
		err = fmt.Errorf("unknown distributor configuration: %q", cfgName)
		return
	}
	if alias := cfg.GetAlias(); alias != nil {
		cfg, ok = distCfg.DistributorConfigs[alias.OtherConfig]
		if !ok {
			err = fmt.Errorf("unknown distributor configuration: %q (via alias %q)", cfgName, alias.OtherConfig)
			return
		}
		if cfg.GetAlias() != nil {
			err = fmt.Errorf("too many levels of indirection for alias %q (points to alias %q)", cfgName, alias.OtherConfig)
			return
		}
	}

	dt := cfg.DistributorType
	if dt == nil {
		err = fmt.Errorf("blank or unrecognized distributor_type")
		return
	}
	dVal := reflect.ValueOf(dt)

	// All non-nil DistributorType's have a single field which is the actual oneof
	// value.
	implConfig := dVal.Elem().Field(0).Interface().(proto.Message)

	ret = &Config{
		info.Get(c).DefaultVersionHostname(),
		cfgName,
		cfgVersion,
		implConfig,
	}
	return
}
