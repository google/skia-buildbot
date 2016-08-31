// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bootstrap

import (
	"errors"
	"fmt"

	"github.com/luci/luci-go/client/environ"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamclient"
	"github.com/luci/luci-go/logdog/common/types"
)

// ErrNotBootstrapped is returned by Get when the current process is not
// bootstrapped.
var ErrNotBootstrapped = errors.New("not bootstrapped")

// Bootstrap contains information about the
type Bootstrap struct {
	// Project is the Butler instance project name.
	Project config.ProjectName
	// Prefix is the Butler instance prefix.
	Prefix types.StreamName

	// Client is the streamclient for this instance, or nil if the Butler has no
	// streamserver.
	Client streamclient.Client
}

func getFromEnv(env environ.Environment, reg *streamclient.Registry) (*Bootstrap, error) {
	// Detect Butler by looking for EnvStreamPrefix in the envrironent.
	prefix, ok := env[EnvStreamPrefix]
	if !ok {
		return nil, ErrNotBootstrapped
	}

	bs := &Bootstrap{
		Prefix:  types.StreamName(prefix),
		Project: config.ProjectName(env[EnvStreamProject]),
	}
	if err := bs.Prefix.Validate(); err != nil {
		return nil, fmt.Errorf("bootstrap: failed to validate prefix %q: %s", prefix, err)
	}
	if err := bs.Project.Validate(); err != nil {
		return nil, fmt.Errorf("bootstrap: failed to validate project %q: %s", bs.Project, err)
	}

	// If we have a stream server attached; instantiate a stream Client.
	if p, ok := env[EnvStreamServerPath]; ok {
		c, err := reg.NewClient(p)
		if err != nil {
			return nil, fmt.Errorf("bootstrap: failed to create stream client [%s]: %s", p, err)
		}
		bs.Client = c
	}

	return bs, nil
}

// Get loads a Bootstrap instance from the environment. It will return an error
// if the bootstrap data is invalid, and will return ErrNotBootstrapped if the
// current process is not bootstrapped.
func Get() (*Bootstrap, error) {
	return getFromEnv(environ.Get(), streamclient.DefaultRegistry)
}
