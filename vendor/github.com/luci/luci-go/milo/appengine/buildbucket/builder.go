// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbucket

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"

	"github.com/luci/luci-go/common/api/buildbucket/buildbucket/v1"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/retry"
	"github.com/luci/luci-go/common/sync/parallel"
	"github.com/luci/luci-go/milo/api/resp"
)

// search executes the search request with retries and exponential back-off.
func search(c context.Context, client *buildbucket.Service, req *buildbucket.SearchCall) (
	*buildbucket.ApiSearchResponseMessage, error) {

	var res *buildbucket.ApiSearchResponseMessage
	err := retry.Retry(
		c,
		retry.TransientOnly(retry.Default),
		func() error {
			var err error
			res, err = req.Do()
			if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code >= 500 {
				err = errors.WrapTransient(apiErr)
			}
			return err
		},
		func(err error, wait time.Duration) {
			log.WithError(err).Warningf(c, "buildbucket search request failed transiently, will retry in %s", wait)
		})
	return res, err
}

// fetchBuilds fetches builds given a criteria.
// The returned builds are sorted by build creation descending.
// count defines maximum number of builds to fetch; if <0, defaults to 100.
func fetchBuilds(c context.Context, client *buildbucket.Service, bucket, builder,
	status string, count int) ([]*buildbucket.ApiBuildMessage, error) {

	req := client.Search()
	req.Bucket(bucket)
	req.Status(status)
	req.Tag("builder:" + builder)

	if count < 0 {
		count = 100
	}

	fetched := make([]*buildbucket.ApiBuildMessage, 0, count)
	start := clock.Now(c)
	for len(fetched) < count {
		req.MaxBuilds(int64(count - len(fetched)))

		res, err := search(c, client, req)
		switch {
		case err != nil:
			return fetched, err
		case res.Error != nil:
			return fetched, fmt.Errorf(res.Error.Message)
		}

		fetched = append(fetched, res.Builds...)

		if len(res.Builds) == 0 || res.NextCursor == "" {
			break
		}
		req.StartCursor(res.NextCursor)
	}
	log.Debugf(c, "Fetched %d %s builds in %s", len(fetched), status, clock.Since(c, start))
	return fetched, nil
}

// toMiloBuild converts a buildbucket build to a milo build.
// In case of an error, returns a build with a description of the error
// and logs the error.
func toMiloBuild(c context.Context, build *buildbucket.ApiBuildMessage) *resp.BuildSummary {
	// Parsing of parameters and result details is best effort.
	var params buildParameters
	if err := json.NewDecoder(strings.NewReader(build.ParametersJson)).Decode(&params); err != nil {
		log.Errorf(c, "Could not parse parameters of build %d: %s", build.Id, err)
	}
	var resultDetails resultDetails
	if err := json.NewDecoder(strings.NewReader(build.ResultDetailsJson)).Decode(&resultDetails); err != nil {
		log.Errorf(c, "Could not parse result details of build %d: %s", build.Id, err)
	}

	result := &resp.BuildSummary{
		Text:     []string{fmt.Sprintf("buildbucket id %d", build.Id)},
		Revision: resultDetails.Properties.GotRevision,
	}
	if result.Revision == "" {
		result.Revision = params.Properties.Revision
	}

	var err error
	result.Status, err = parseStatus(build)
	if err != nil {
		// almost never happens
		log.WithError(err).Errorf(c, "could not convert status of build %d", build.Id)
		result.Status = resp.InfraFailure
		result.Text = append(result.Text, fmt.Sprintf("invalid build: %s", err))
	}

	result.PendingTime.Started = parseTimestamp(build.CreatedTs)
	switch build.Status {
	case "SCHEDULED":
		result.PendingTime.Duration = clock.Since(c, result.PendingTime.Started)

	case "STARTED":
		result.ExecutionTime.Started = parseTimestamp(build.StatusChangedTs)
		result.ExecutionTime.Duration = clock.Since(c, result.PendingTime.Started)
		result.PendingTime.Finished = result.ExecutionTime.Started
		result.PendingTime.Duration = result.PendingTime.Finished.Sub(result.PendingTime.Started)

	case "COMPLETED":
		// buildbucket does not provide build start time or execution duration.
		result.ExecutionTime.Finished = parseTimestamp(build.CompletedTs)
	}

	cl := getChangeList(build, &params, &resultDetails)
	if cl != nil {
		result.Blame = []*resp.Commit{cl}
	}

	tags := ParseTags(build.Tags)

	if build.Url != "" {
		u := build.Url
		parsed, err := url.Parse(u)

		// map milo links to itself
		switch {
		case err != nil:
			log.Errorf(c, "invalid URL in build %d: %s", build.Id, err)
		case parsed.Host == "luci-milo.appspot.com":
			parsed.Host = ""
			parsed.Scheme = ""
			u = parsed.String()
		}

		result.Link = &resp.Link{
			URL:   u,
			Label: strconv.FormatInt(build.Id, 10),
		}

		// compute the best link label
		if taskID := tags["swarming_task_id"]; taskID != "" {
			result.Link.Label = taskID
		} else if resultDetails.Properties.BuildNumber != 0 {
			result.Link.Label = strconv.Itoa(resultDetails.Properties.BuildNumber)
		} else if parsed != nil {
			// does the URL look like a buildbot build URL?
			pattern := fmt.Sprintf(
				`/%s/builders/%s/builds/`,
				strings.TrimPrefix(build.Bucket, "master."), params.BuilderName)
			beforeBuildNumber, buildNumberStr := path.Split(parsed.Path)
			_, err := strconv.Atoi(buildNumberStr)
			if strings.HasSuffix(beforeBuildNumber, pattern) && err == nil {
				result.Link.Label = buildNumberStr
			}
		}
	}

	return result
}

func getDebugBuilds(c context.Context, bucket, builder string, maxCompletedBuilds int, target *resp.Builder) error {
	// ../buildbucket below assumes that
	// - this code is not executed by tests outside of this dir
	// - this dir is a sibling of frontend dir
	resFile, err := os.Open(filepath.Join(
		"..", "buildbucket", "testdata", bucket, builder+".json"))
	if err != nil {
		return err
	}
	defer resFile.Close()

	res := &buildbucket.ApiSearchResponseMessage{}
	if err := json.NewDecoder(resFile).Decode(res); err != nil {
		return err
	}

	for _, bb := range res.Builds {
		mb := toMiloBuild(c, bb)
		switch mb.Status {
		case resp.NotRun:
			target.PendingBuilds = append(target.PendingBuilds, mb)

		case resp.Running:
			target.CurrentBuilds = append(target.CurrentBuilds, mb)

		case resp.Success, resp.Failure, resp.InfraFailure, resp.Warning:
			if len(target.FinishedBuilds) < maxCompletedBuilds {
				target.FinishedBuilds = append(target.FinishedBuilds, mb)
			}

		default:
			panic("impossible")
		}
	}
	return nil
}

// builderImpl is the implementation for getting a milo builder page from buildbucket.
// if maxCompletedBuilds < 0, 25 is used.
func builderImpl(c context.Context, server, bucket, builder string, maxCompletedBuilds int) (*resp.Builder, error) {
	if maxCompletedBuilds < 0 {
		maxCompletedBuilds = 20
	}

	result := &resp.Builder{
		Name: builder,
	}
	if server == "debug" {
		return result, getDebugBuilds(c, bucket, builder, maxCompletedBuilds, result)
	}
	client, err := newClient(c, server)
	if err != nil {
		return nil, err
	}

	fetch := func(target *[]*resp.BuildSummary, status string, count int) error {
		builds, err := fetchBuilds(c, client, bucket, builder, status, count)
		if err != nil {
			log.Errorf(c, "Could not fetch builds with status %s: %s", status, err)
			return err
		}
		*target = make([]*resp.BuildSummary, len(builds))
		for i, bb := range builds {
			(*target)[i] = toMiloBuild(c, bb)
		}
		return nil
	}
	// fetch pending, current and finished builds concurrently.
	// Why not a single request? Because we need different build number
	// limits for different statuses.
	return result, parallel.FanOutIn(func(work chan<- func() error) {
		work <- func() error {
			return fetch(&result.PendingBuilds, StatusScheduled, -1)
		}
		work <- func() error {
			return fetch(&result.CurrentBuilds, StatusStarted, -1)
		}
		work <- func() error {
			return fetch(&result.FinishedBuilds, StatusCompleted, maxCompletedBuilds)
		}
	})
}

// parseTimestamp converts buildbucket timestamp in microseconds to time.Time
func parseTimestamp(microseconds int64) time.Time {
	if microseconds == 0 {
		return time.Time{}
	}
	return time.Unix(microseconds/1e6, microseconds%1e6*1000).UTC()
}

type newBuildsFirst []*resp.BuildSummary

func (a newBuildsFirst) Len() int      { return len(a) }
func (a newBuildsFirst) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a newBuildsFirst) Less(i, j int) bool {
	return a[i].PendingTime.Started.After(a[j].PendingTime.Started)
}
