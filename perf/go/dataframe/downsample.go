package dataframe

import (
	"math"

	"go.skia.org/infra/go/vcsinfo"
)

// DownSample the given slice of IndexCommits so that there's no more than 'n'
// IndexCommits returned.
func DownSample(sample []*vcsinfo.IndexCommit, n int) []*vcsinfo.IndexCommit {
	if len(sample) < 2 {
		return sample
	}
	if n <= 0 {
		return sample
	}
	skip := int(math.Ceil(float64(len(sample)) / float64(n)))
	ret := []*vcsinfo.IndexCommit{}
	for i := 0; i < len(sample); i += skip {
		ret = append(ret, sample[i])
	}
	return ret
}
