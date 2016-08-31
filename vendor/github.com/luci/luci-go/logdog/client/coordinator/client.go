// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/grpc/prpc"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/common/types"
)

var (
	// Scopes is the set of scopes needed for the Coordinator user endpoints.
	Scopes = []string{
		auth.OAuthScopeEmail,
	}
)

// Client wraps a Logs client with user-friendly methods.
//
// Each method should operate independently, so calling methods from different
// goroutines must not cause any problems.
type Client struct {
	// C is the underlying LogsClient interface.
	C logdog.LogsClient
}

// NewClient returns a new Client instance bound to a pRPC Client.
func NewClient(c *prpc.Client) *Client {
	return &Client{
		C: logdog.NewLogsPRPCClient(c),
	}
}

// Stream returns a Stream instance for the named stream.
func (c *Client) Stream(project config.ProjectName, path types.StreamPath) *Stream {
	return &Stream{
		c:       c,
		project: project,
		path:    path,
	}
}
