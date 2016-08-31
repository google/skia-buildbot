// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package swarming

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/milo/api/resp"
	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/server/templates"
)

var testCases = []struct {
	input        string
	expectations string
}{
	{"build-canceled", "build-canceled.json"},
	{"build-exception", "build-exception.json"},
	{"build-hang", "build-hang.json"},
	{"build-link", "build-link.json"},
	{"build-patch-failure", "build-patch-failure.json"},
	{"build-pending", "build-pending.json"},
	{"build-running", "build-running.json"},
	{"build-timeout", "build-timeout.json"},
	{"build-unicode", "build-unicode.json"},
}

func getTestCases() []string {
	testdata := filepath.Join("..", "swarming", "testdata")
	results := []string{}
	f, err := ioutil.ReadDir(testdata)
	if err != nil {
		panic(err)
	}
	for _, fi := range f {
		if strings.HasSuffix(fi.Name(), ".swarm") {
			results = append(results, fi.Name()[0:len(fi.Name())-6])
		}
	}
	return results
}

// TestableLog is a subclass of Log that interfaces with TestableHandler and
// includes sample test data.
type TestableLog struct{ Log }

// TestableBuild is a subclass of Build that interfaces with TestableHandler and
// includes sample test data.
type TestableBuild struct{ Build }

// TestData returns sample test data.
func (l TestableLog) TestData() []settings.TestBundle {
	return []settings.TestBundle{
		{
			Description: "Basic log",
			Data: templates.Args{
				"Log":    "This is the log",
				"Closed": true,
			},
		},
	}
}

// TestData returns sample test data.
func (b TestableBuild) TestData() []settings.TestBundle {
	basic := resp.MiloBuild{
		Summary: resp.BuildComponent{
			Label:    "Test swarming build",
			Status:   resp.Success,
			Started:  time.Date(2016, 1, 2, 15, 4, 5, 999999999, time.UTC),
			Finished: time.Date(2016, 1, 2, 15, 4, 6, 999999999, time.UTC),
			Duration: time.Second,
		},
	}
	results := []settings.TestBundle{
		{
			Description: "Basic successful build",
			Data:        templates.Args{"Build": basic},
		},
	}
	c := context.Background()
	c, _ = testclock.UseTime(c, time.Date(2016, time.March, 14, 11, 0, 0, 0, time.UTC))
	for _, tc := range getTestCases() {
		build, err := swarmingBuildImpl(c, "foo", "debug", tc)
		if err != nil {
			panic(fmt.Errorf("Error while processing %s: %s", tc, err))
		}
		results = append(results, settings.TestBundle{
			Description: tc,
			Data:        templates.Args{"Build": build},
		})
	}
	return results
}
