// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package logdog

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock"
	miloProto "github.com/luci/luci-go/common/proto/milo"
	"github.com/luci/luci-go/milo/api/resp"
)

// URLBuilder constructs URLs for various link types.
type URLBuilder interface {
	// LinkURL returns the URL associated with the supplied Link.
	//
	// If no URL could be built for that Link, nil will be returned.
	BuildLink(l *miloProto.Link) *resp.Link
}

// HACK(hinoka): This should be a part of recipes, but just hardcoding a list
// of unimportant things for now.
var builtIn = map[string]struct{}{
	"recipe bootstrap": {},
	"setup_build":      {},
	"recipe result":    {},
}

// miloBuildStep converts a logdog/milo step to a BuildComponent struct.
// buildCompletedTime must be zero if build did not complete yet.
func miloBuildStep(ub URLBuilder, anno *miloProto.Step, buildCompletedTime, now time.Time) *resp.BuildComponent {

	comp := &resp.BuildComponent{Label: anno.Name}
	switch anno.Status {
	case miloProto.Status_RUNNING:
		comp.Status = resp.Running

	case miloProto.Status_SUCCESS:
		comp.Status = resp.Success

	case miloProto.Status_FAILURE:
		if fd := anno.GetFailureDetails(); fd != nil {
			switch fd.Type {
			case miloProto.FailureDetails_EXCEPTION, miloProto.FailureDetails_INFRA:
				comp.Status = resp.InfraFailure

			case miloProto.FailureDetails_DM_DEPENDENCY_FAILED:
				comp.Status = resp.DependencyFailure

			default:
				comp.Status = resp.Failure
			}

			if fd.Text != "" {
				comp.Text = append(comp.Text, fd.Text)
			}
		} else {
			comp.Status = resp.Failure
		}

	case miloProto.Status_PENDING:
		comp.Status = resp.NotRun

		// Missing the case of waiting on unfinished dependency...
	default:
		comp.Status = resp.NotRun
	}

	if !(buildCompletedTime.IsZero() || comp.Status.Terminal()) {
		// The build has completed, but this step has not. Mark it as an
		// infrastructure failure.
		comp.Status = resp.InfraFailure
	}

	// Hide the unimportant steps, highlight the interesting ones.
	switch comp.Status {
	case resp.Success:
		if _, ok := builtIn[anno.Name]; ok {
			comp.Verbosity = resp.Hidden
		}
	case resp.InfraFailure, resp.Failure:
		comp.Verbosity = resp.Interesting
	}

	// Main link is a link to the stdout.
	var stdoutLink *miloProto.Link
	if anno.StdoutStream != nil {
		stdoutLink = &miloProto.Link{
			Label: "stdout",
			Value: &miloProto.Link_LogdogStream{
				LogdogStream: anno.StdoutStream,
			},
		}
	}

	if anno.Link != nil {
		comp.MainLink = ub.BuildLink(anno.Link)

		// If we also have a STDOUT stream, add it to our OtherLinks.
		if stdoutLink != nil {
			anno.OtherLinks = append([]*miloProto.Link{stdoutLink}, anno.OtherLinks...)
		}
	} else if stdoutLink != nil {
		comp.MainLink = ub.BuildLink(stdoutLink)
	}

	// Sub link is for one link per log that isn't stdout.
	for _, link := range anno.GetOtherLinks() {
		if l := ub.BuildLink(link); l != nil {
			comp.SubLink = append(comp.SubLink, l)
		}
	}

	// This should always be a step.
	comp.Type = resp.Step

	// This should always be 0
	comp.LevelsDeep = 0

	// Timestamps
	comp.Started = anno.Started.Time()
	comp.Finished = anno.Ended.Time()

	var till time.Time
	switch {
	case !comp.Finished.IsZero():
		till = comp.Finished
	case comp.Status == resp.Running:
		till = now
	case !buildCompletedTime.IsZero():
		till = buildCompletedTime
	}
	if !comp.Started.IsZero() && till.After(comp.Started) {
		comp.Duration = till.Sub(comp.Started)
	}

	// This should be the exact same thing.
	comp.Text = append(comp.Text, anno.Text...)

	return comp
}

// AddLogDogToBuild takes a set of logdog streams and populate a milo build.
// build.Summary.Finished must be set.
func AddLogDogToBuild(c context.Context, ub URLBuilder, s *Streams, build *resp.MiloBuild) {
	if s.MainStream == nil {
		panic("missing main stream")
	}
	// Now Fetch the main annotation of the build.
	var (
		mainAnno = s.MainStream.Data
		now      = clock.Now(c)
	)

	// Now fill in each of the step components.
	// TODO(hinoka): This is totes cachable.
	buildCompletedTime := mainAnno.Ended.Time()
	build.Summary = *(miloBuildStep(ub, mainAnno, buildCompletedTime, now))
	for _, substepContainer := range mainAnno.Substep {
		anno := substepContainer.GetStep()
		if anno == nil {
			// TODO: We ignore non-embedded substeps for now.
			continue
		}

		bs := miloBuildStep(ub, anno, buildCompletedTime, now)
		if bs.Status != resp.Success {
			build.Summary.Text = append(
				build.Summary.Text, fmt.Sprintf("%s %s", bs.Status, bs.Label))
		}
		build.Components = append(build.Components, bs)
		propGroup := &resp.PropertyGroup{GroupName: bs.Label}
		for _, prop := range anno.Property {
			propGroup.Property = append(propGroup.Property, &resp.Property{
				Key:   prop.Name,
				Value: prop.Value,
			})
		}
		build.PropertyGroup = append(build.PropertyGroup, propGroup)
	}

	// Take care of properties
	propGroup := &resp.PropertyGroup{GroupName: "Main"}
	for _, prop := range mainAnno.Property {
		propGroup.Property = append(propGroup.Property, &resp.Property{
			Key:   prop.Name,
			Value: prop.Value,
		})
	}
	build.PropertyGroup = append(build.PropertyGroup, propGroup)

	return
}
