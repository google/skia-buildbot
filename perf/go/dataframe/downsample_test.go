package dataframe

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/vcsinfo"
)

func TestDownSample(t *testing.T) {
	sample := []*vcsinfo.IndexCommit{}
	for i := 0; i < 5; i++ {
		sample = append(sample, &vcsinfo.IndexCommit{Index: i})
	}

	ss := DownSample(sample, 5)
	assert.Equal(t, 5, len(ss))

	ss = DownSample(sample, 6)
	assert.Equal(t, 5, len(ss))

	ss = DownSample(sample, 3)
	assert.Equal(t, 3, len(ss))
	assert.Equal(t, 2, ss[1].Index)
	assert.Equal(t, 4, ss[2].Index)

	ss = DownSample(sample, 1)
	assert.Equal(t, 1, len(ss))

	ss = DownSample(sample[:1], 1)
	assert.Equal(t, 1, len(ss))

	ss = DownSample([]*vcsinfo.IndexCommit{}, 1)
	assert.Equal(t, 0, len(ss))

	ss = DownSample(sample, 0)
	assert.Equal(t, 5, len(ss))

	ss = DownSample(sample, 4)
	assert.Equal(t, 3, len(ss))
}
