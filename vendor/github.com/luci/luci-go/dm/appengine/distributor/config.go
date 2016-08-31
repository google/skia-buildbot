// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package distributor

import (
	"golang.org/x/net/context"

	"github.com/golang/protobuf/proto"
	"github.com/luci/gae/service/taskqueue"
)

// Config represents the configuration for a single instance of a given
// distributor implementation at a given point in time (e.g. version).
type Config struct {
	// DMHost is the host for the DM API. This may be used by the distributor
	// implementation to pass to jobs so that they can call back into DM's api.
	DMHost string

	// Name is the name of this distributor configuration. This is always the
	// fully-resolved name of the configuration (i.e. aliases are dereferenced).
	Name string

	// Version is the version of the distributor configuration retrieved from
	// luci-config.
	Version string

	// Content is the actual parsed implementation-specific configuration.
	Content proto.Message
}

// EnqueueTask allows a Distributor to enqueue a TaskQueue task that will be
// handled by the Distributor's HandleTaskQueueTask method.
func (cfg *Config) EnqueueTask(c context.Context, tsk *taskqueue.Task) error {
	tsk.Path = handlerPath(cfg.Name)
	return taskqueue.Get(c).Add(tsk, "")
}
