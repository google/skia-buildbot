package revision_filter

import (
	"context"
	"fmt"
	"net/http"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// BuildbucketRevisionFilter is a RevisionFilter which uses results from
// BuildBucket to filter Revisions.
type BuildbucketRevisionFilter struct {
	bb       buildbucket.BuildBucketInterface
	bbConfig *config.BuildbucketRevisionFilterConfig
}

// Skip implements RevisionFilter.
func (f BuildbucketRevisionFilter) Skip(ctx context.Context, r *revision.Revision) (string, error) {
	pred := &buildbucketpb.BuildPredicate{
		Builder: &buildbucketpb.BuilderID{Project: f.bbConfig.Project, Bucket: f.bbConfig.Bucket},
		Tags: []*buildbucketpb.StringPair{
			{Key: "buildset", Value: fmt.Sprintf(f.bbConfig.BuildsetCommitTmpl, r.Id)},
		},
	}
	builds, err := f.bb.Search(ctx, pred)
	if err != nil {
		return "", err
	}
	if len(builds) == 0 {
		sklog.Infof("[bbFilter] Builds for %s have not started yet", r.Id)
		return "Builds have not started yet", nil
	}

	// statuses stores the statuses of builders. This is used to account for luci build retries.
	// It is used to determine if there was any successful build for a builder. We should have ideally used
	// the most recent status but there appears to be strange behavior with flutter luci builds where
	// INFRA_FAILURE builds appear to be coming after SUCCESSFUL builds. Eg:
	// https://cr-buildbucket.appspot.com/rpcexplorer/services/buildbucket.v2.Builds/SearchBuilds?request={"predicate":{"builder":{"project": "flutter","bucket": "prod"},"tags":[{"key": "buildset","value": "commit/git/18962926012965f815c273e58409cda3144998f5"}]}}
	// This has been brought up with the flutter team.
	statuses := map[string]buildbucketpb.Status{}
	for _, build := range builds {
		prev, ok := statuses[build.Builder.Builder]
		if !ok || prev != buildbucketpb.Status_SUCCESS {
			statuses[build.Builder.Builder] = build.Status
		}
	}
	for b, status := range statuses {
		if status == buildbucketpb.Status_SUCCESS {
			sklog.Infof("[bbFilter] Found successful build of \"%s\" for %s", b, r.Id)
		} else {
			sklog.Infof("[bbFilter] Could not find successful build of \"%s\" for %s: %s", b, r.Id, status)
			return fmt.Sprintf("Luci builds of \"%s\" for %s was %s", b, r.Id, status), nil
		}
	}
	sklog.Infof("[bbFilter] All builds of %s were %s", r.Id, buildbucketpb.Status_SUCCESS)
	return "", nil
}

// NewBuildbucketRevisionFilter returns a RevisionFilter which uses results from
// Buildbucket to filter revisions.
func NewBuildbucketRevisionFilter(client *http.Client, bbConfig *config.BuildbucketRevisionFilterConfig) (*BuildbucketRevisionFilter, error) {
	if bbConfig == nil {
		return nil, skerr.Fmt("BuildbucketRevisionFilterConfig config must be specified")
	}
	return &BuildbucketRevisionFilter{
		bb:       buildbucket.NewClient(client),
		bbConfig: bbConfig,
	}, nil
}

// bbRevisionFilter implements RevisionFilter.
var _ RevisionFilter = &BuildbucketRevisionFilter{}
