// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package buildbot

import (
	"fmt"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/luci-go/common/clock/testclock"
	"github.com/luci/luci-go/milo/api/resp"
	"github.com/luci/luci-go/milo/appengine/settings"
	"github.com/luci/luci-go/server/templates"
	"golang.org/x/net/context"
)

// We put this here because _test.go files are sometimes not built.
var testCases = []struct {
	builder string
	build   string
}{
	{"CrWinGoma", "30608"},
	{"win_chromium_rel_ng", "246309"},
}

// TestableBuild is a subclass of Build that interfaces with TestableHandler and
// includes sample test data.
type TestableBuild struct{ Build }

// TestableBuilder is a subclass of Builder that interfaces with TestableHandler
// and includes sample test data.
type TestableBuilder struct{ Builder }

// TestData returns sample test data.
func (b Build) TestData() []settings.TestBundle {
	c := memory.Use(context.Background())
	c, _ = testclock.UseTime(c, testclock.TestTimeUTC)
	bundles := []settings.TestBundle{}
	for _, tc := range testCases {
		build, err := build(c, "debug", tc.builder, tc.build)
		if err != nil {
			panic(fmt.Errorf(
				"Encountered error while building debug/%s/%s.\n%s",
				tc.builder, tc.build, err))
		}
		bundles = append(bundles, settings.TestBundle{
			Description: fmt.Sprintf("Debug page: %s/%s", tc.builder, tc.build),
			Data: templates.Args{
				"Build": build,
			},
		})
	}
	return bundles
}

// TestData returns sample test data.
func (b Builder) TestData() []settings.TestBundle {
	return []settings.TestBundle{
		{
			Description: "Basic Test no builds",
			Data: templates.Args{
				"Builder": &resp.Builder{
					Name: "Sample Builder",
				},
			},
		},
		{
			Description: "Basic Test with builds",
			Data: templates.Args{
				"Builder": &resp.Builder{
					Name: "Sample Builder",
					CurrentBuilds: []*resp.BuildSummary{
						{
							Link: &resp.Link{
								URL:   "https://some.url/path",
								Label: "Some current build",
							},
							Revision: "deadbeef",
						},
					},
					PendingBuilds: []*resp.BuildSummary{
						{
							Link: &resp.Link{
								URL:   "https://some.url/path",
								Label: "Some current build",
							},
							Revision: "deadbeef",
						},
					},
					FinishedBuilds: []*resp.BuildSummary{
						{
							Link: &resp.Link{
								URL:   "https://some.url/path",
								Label: "Some current build",
							},
							Revision: "deadbeef",
						},
					},
				},
			},
		},
	}
}
