package common

import (
	"fmt"
	"net/url"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"google.golang.org/genproto/protobuf/field_mask"
)

// common variables that need to be exposed to the mock
// External clients should not use these

var (
	// GetBuildFields is a FieldMask which indicates which fields we want
	// returned from a GetBuild request.
	GetBuildFields = &field_mask.FieldMask{
		Paths: []string{
			"id",
			"builder",
			"created_by",
			"create_time",
			"start_time",
			"end_time",
			"status",
			"input.properties",
		},
	}

	// SearchBuildsFields is a FieldMask which indicates which fields we
	// want returned from a SearchBuilds request.
	SearchBuildsFields = &field_mask.FieldMask{
		Paths: func(buildFields []string) []string {
			rv := make([]string, 0, len(buildFields))
			for _, f := range buildFields {
				rv = append(rv, fmt.Sprintf("builds.*.%s", f))
			}
			return rv
		}(GetBuildFields.Paths),
	}
)

// GetTrybotsForCLPredicate returns a *buildbucketpb.BuildPredicate which
// searches for trybots from the given CL.
func GetTrybotsForCLPredicate(issue, patchset int64, gerritUrl string) (*buildbucketpb.BuildPredicate, error) {
	u, err := url.Parse(gerritUrl)
	if err != nil {
		return nil, err
	}
	return &buildbucketpb.BuildPredicate{
		GerritChanges: []*buildbucketpb.GerritChange{
			{
				Host:     u.Host,
				Change:   issue,
				Patchset: patchset,
			},
		},
	}, nil
}
