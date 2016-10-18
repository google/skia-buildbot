package dataframe

import (
	"math"

	"go.skia.org/infra/go/vcsinfo"
)

// DownSample the given slice of IndexCommits so that there's no more than 'n'
// IndexCommits returned.
//
// Returns the downsamples IndexCommits and the number of commits that
// were skipped. I.e. if N is returned then the returned slice will contain
// every (N+1)th commit.
func DownSample(sample []*vcsinfo.IndexCommit, n int) ([]*vcsinfo.IndexCommit, int) {
	if len(sample) <= n {
		return sample, 0
	}
	if n <= 0 {
		return sample, 0
	}
	skip := int(math.Ceil(float64(len(sample)) / float64(n)))
	ret := []*vcsinfo.IndexCommit{}
	for i := 0; i < len(sample); i += skip {
		ret = append(ret, sample[i])
	}
	return ret, skip - 1
}
