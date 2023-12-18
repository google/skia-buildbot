package urlprovider

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/types"
)

type URLProvider struct {
	perfGit        perfgit.Git
	paramsProvider ParamsProvider
}

// Explore generates a url to the explore page for the given parameters
func (prov *URLProvider) Explore(ctx context.Context, startCommitNumber int, endCommitNumber int, parameters map[string]util.StringSet) string {
	queryUrl := url.Values{}
	startCommit, err := prov.perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(startCommitNumber))
	if err != nil {
		sklog.Error("Error getting commit info")
	}
	endCommit, _ := prov.perfGit.CommitFromCommitNumber(ctx, types.CommitNumber(endCommitNumber))
	queryUrl["begin"] = []string{strconv.Itoa(int(startCommit.Timestamp))}

	// We will shift the end time by a day so the graph doesn't render the anomalies right at the end
	endTime := time.Unix(endCommit.Timestamp, 0).AddDate(0, 0, 1)
	queryUrl["end"] = []string{strconv.Itoa(int(endTime.Unix()))}

	queryUrl["summary"] = []string{"true"}
	// Now let's look at the parameters for the query
	query_portion := url.Values{}
	for paramName, paramValues := range parameters {
		paramKey := prov.paramsProvider.GetParamKey(paramName)
		if paramKey != "" {
			query_portion[paramKey] = paramValues.Keys()
		}
	}

	queryUrl["queries"] = []string{query_portion.Encode()}

	return "/e/?" + queryUrl.Encode()
}

// New creates a new instance of the UrlProvider struct
func New(perfgit perfgit.Git, paramsProvider ParamsProvider) *URLProvider {
	if paramsProvider == nil {
		paramsProvider = &DefaultParamsProvider{}
	}
	return &URLProvider{
		perfGit:        perfgit,
		paramsProvider: paramsProvider,
	}
}
