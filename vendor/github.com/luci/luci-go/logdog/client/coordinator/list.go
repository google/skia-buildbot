// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/logdog/api/endpoints/coordinator/logs/v1"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

// ListResult is a single returned list entry.
type ListResult struct {
	// Project is the project that this result is bound to.
	Project config.ProjectName
	// PathBase is the base part of the list path. This will match the base that
	// the list was pulled from.
	PathBase types.StreamPath
	// Name is the name of this component, relative to PathBase.
	Name string
	// Stream is true if this list component is a stream component. Otherwise,
	// it's an intermediate path component.
	Stream bool

	// State is the state of the log stream. This will be populated if this is a
	// stream component and state was requested.
	State *LogStream
}

// FullPath returns the full StreamPath of this result. If it is not a stream,
// it will return an empty path.
func (lr *ListResult) FullPath() types.StreamPath {
	return types.StreamPath(lr.PathBase).Append(lr.Name)
}

// ListCallback is a callback method type that is used in list requests.
//
// If it returns false, additional callbacks and list requests will be aborted.
type ListCallback func(*ListResult) bool

// ListOptions is a set of list request options.
type ListOptions struct {
	// StreamsOnly requests that only stream components are returned. Intermediate
	// path components will be omitted.
	StreamsOnly bool
	// State, if true, requests that the full log stream descriptor and state get
	// included in returned stream list components.
	State bool

	// Purged, if true, requests that purged log streams are included in the list
	// results. This will result in an error if the user is not privileged to see
	// purged logs.
	Purged bool
}

// List executes a log stream hierarchy listing for the specified path.
//
// If project is the empty string, a top-level project listing will be returned.
func (c *Client) List(ctx context.Context, project config.ProjectName, pathBase string, o ListOptions, cb ListCallback) error {
	req := logdog.ListRequest{
		Project:       string(project),
		PathBase:      pathBase,
		StreamOnly:    o.StreamsOnly,
		State:         o.State,
		IncludePurged: o.Purged,
	}

	for {
		resp, err := c.C.List(ctx, &req)
		if err != nil {
			return normalizeError(err)
		}

		for _, s := range resp.Components {
			lr := ListResult{
				Project:  config.ProjectName(resp.Project),
				PathBase: types.StreamPath(resp.PathBase),
				Name:     s.Name,
			}
			switch s.Type {
			case logdog.ListResponse_Component_PATH:
				break

			case logdog.ListResponse_Component_STREAM:
				lr.Stream = true
				if s.State != nil {
					lr.State = loadLogStream(resp.Project, lr.FullPath(), s.State, s.Desc)
				}

			case logdog.ListResponse_Component_PROJECT:
				lr.Project = config.ProjectName(lr.Name)
			}

			if !cb(&lr) {
				return nil
			}
		}

		if resp.Next == "" {
			return nil
		}
		req.Next = resp.Next
	}
}
