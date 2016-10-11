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

	s0 := DownSample(sample, 5)
	assert.Equal(t, 5, len(s0))

	s1 := DownSample(sample, 6)
	assert.Equal(t, 5, len(s1))

	s2 := DownSample(sample, 3)
	assert.Equal(t, 3, len(s2))
	assert.Equal(t, 2, s2[1].Index)
	assert.Equal(t, 4, s2[2].Index)

	s3 := DownSample(sample, 1)
	assert.Equal(t, 1, len(s3))

	s4 := DownSample(sample[:1], 1)
	assert.Equal(t, 1, len(s4))

	s5 := DownSample([]*vcsinfo.IndexCommit{}, 1)
	assert.Equal(t, 0, len(s5))
}
