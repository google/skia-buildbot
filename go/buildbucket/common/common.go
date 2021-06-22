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
	// TODO(borenet): This is the union of all fields needed by all users
	// of GetBuild. We should use a separate Field Mask per use case.
	GetBuildFields = &field_mask.FieldMask{
		Paths: []string{
			"builder",
			"create_time",
			"created_by",
			"end_time",
			"id",
			"input.gerrit_changes",
			"input.properties",
			"start_time",
			"status",
			"tags",
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
func GetTrybotsForCLPredicate(issue, patchset int64, gerritUrl string, tags map[string]string) (*buildbucketpb.BuildPredicate, error) {
	u, err := url.Parse(gerritUrl)
	if err != nil {
		return nil, err
	}
	tagStringPairs := []*buildbucketpb.StringPair{}
	for k, v := range tags {
		stringPair := &buildbucketpb.StringPair{
			Key:   k,
			Value: v,
		}
		tagStringPairs = append(tagStringPairs, stringPair)
	}
	return &buildbucketpb.BuildPredicate{
		GerritChanges: []*buildbucketpb.GerritChange{
			{
				Host:     u.Host,
				Change:   issue,
				Patchset: patchset,
			},
		},
		Tags: tagStringPairs,
	}, nil
}
