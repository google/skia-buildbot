// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbot

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/luci/luci-go/grpc/prpc/talk/buildbot/proto"
)

type buildbotService struct{}

func (s *buildbotService) Search(c context.Context, req *buildbot.SearchRequest) (*buildbot.SearchResponse, error) {
	allBuilds := []*buildbot.Build{
		{
			Master:  "master.tryserver.chromium.linux",
			Builder: "Debug",
			Number:  1,
			State:   buildbot.BuildState_PENDING,
		},
		{
			Master:  "master.tryserver.chromium.linux",
			Builder: "Release",
			Number:  1,
			State:   buildbot.BuildState_RUNNING,
		},
		{
			Master:  "master.tryserver.chromium.win",
			Builder: "Debug",
			Number:  1,
			State:   buildbot.BuildState_SUCCESS,
		},
	}
	res := &buildbot.SearchResponse{}
	for _, b := range allBuilds {
		if req.Master != "" && b.Master != "" {
			continue
		}
		if req.State != buildbot.BuildState_UNSET && b.State != req.State {
			continue
		}
		if req.Builder != "" && b.Builder != req.Builder {
			continue
		}
		res.Builds = append(res.Builds, b)
	}
	return res, nil
}

func (s *buildbotService) Schedule(c context.Context, req *buildbot.ScheduleRequest) (*buildbot.ScheduleResponse, error) {
	if req.Master == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "Master not specified")
	}

	res := &buildbot.ScheduleResponse{}
	for i, b := range req.GetBuilds() {
		if b.Builder == "" {
			return nil, grpc.Errorf(codes.InvalidArgument, "Builder not specified")
		}

		res.Builds = append(res.Builds, &buildbot.Build{
			Master:  req.Master,
			Builder: b.Builder,
			Number:  int32(i) + 1,
			State:   buildbot.BuildState_PENDING,
		})
	}
	return res, nil
}
