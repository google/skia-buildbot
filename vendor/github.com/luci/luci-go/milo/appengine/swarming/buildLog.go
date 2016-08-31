// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package swarming

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/memcache"
	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/luci/luci-go/common/logging"
)

// swarmingBuildLogImpl is the implementation for getting a log name from
// a swarming build via annotee.  It returns the full text of the specific log,
// and whether or not it has been closed.
func swarmingBuildLogImpl(c context.Context, server, taskID, logname string) (string, bool, error) {
	mc := memcache.Get(c)
	cached, err := mc.Get(path.Join("swarmingLog", server, taskID, logname))
	switch {
	case err == memcache.ErrCacheMiss:

	case err != nil:
		logging.WithError(err).Errorf(c, "failed to fetch log with key %s from memcache", cached.Key())

	default:
		logging.Debugf(c, "Cache hit for step log %s/%s/%s", server, taskID, logname)
		return string(cached.Value()), false, nil
	}

	var sc *swarming.Service
	debug := strings.HasPrefix(taskID, "debug:")
	if debug {
		// if taskID starts with "debug:", then getTaskOutput will ignore client.
	} else {
		var err error
		sc, err = getSwarmingClient(c, server)
		if err != nil {
			return "", false, err
		}
	}

	output, err := getTaskOutput(sc, taskID)
	if err != nil {
		return "", false, err
	}

	// Decode the data using annotee.
	s, err := streamsFromAnnotatedLog(c, output)
	if err != nil {
		return "", false, err
	}

	k := fmt.Sprintf("steps%s", logname)
	stream, ok := s.Streams[k]
	if !ok {
		var keys []string
		for sk := range s.Streams {
			keys = append(keys, sk)
		}
		sort.Strings(keys)
		return "", false, fmt.Errorf("stream %q not found; available streams: %q", k, keys)
	}

	if stream.Closed && !debug {
		cached.SetValue([]byte(stream.Text))
		if err := mc.Set(cached); err != nil {
			logging.Errorf(c, "Failed to write log with key %s to memcache: %s", cached.Key(), err)
		}
	}

	return stream.Text, stream.Closed, nil
}
