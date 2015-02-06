package analysis

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"skia.googlesource.com/buildbot.git/golden/go/types"
	ptypes "skia.googlesource.com/buildbot.git/perf/go/types"
)

func init() {
}

func TestGetBlameList(t *testing.T) {
	start := time.Now().Unix()
	commits := []*ptypes.Commit{
		&ptypes.Commit{CommitTime: start + 10, Hash: "h1", Author: "John Doe 1"},
		&ptypes.Commit{CommitTime: start + 20, Hash: "h2", Author: "John Doe 2"},
		&ptypes.Commit{CommitTime: start + 30, Hash: "h3", Author: "John Doe 3"},
		&ptypes.Commit{CommitTime: start + 40, Hash: "h4", Author: "John Doe 4"},
		&ptypes.Commit{CommitTime: start + 50, Hash: "h5", Author: "John Doe 5"},
	}

	un, pos, neg := types.UNTRIAGED, types.POSITIVE, types.NEGATIVE
	d1_un, d2_pos, d3_neg := "digest1", "digest2", "digest3"

	traces := map[string][]*LabeledTrace{
		"test1": []*LabeledTrace{
			&LabeledTrace{
				CommitIds: []int{2},
				Digests:   []string{d1_un},
				Labels:    []types.Label{un},
			},
			&LabeledTrace{
				CommitIds: []int{1, 3},
				Digests:   []string{d1_un, d2_pos},
				Labels:    []types.Label{un, pos},
			},
			&LabeledTrace{
				CommitIds: []int{0, 3},
				Digests:   []string{d2_pos, d1_un},
				Labels:    []types.Label{pos, un},
			},
		},
		"test2": []*LabeledTrace{
			&LabeledTrace{
				CommitIds: []int{0, 1, 2, 3, 4},
				Digests:   []string{d1_un, d2_pos, d1_un, d3_neg, d3_neg},
				Labels:    []types.Label{un, pos, un, neg, neg},
			},
			&LabeledTrace{
				CommitIds: []int{3, 4},
				Digests:   []string{d1_un, d2_pos},
				Labels:    []types.Label{un, pos},
			},
			&LabeledTrace{
				CommitIds: []int{1, 2, 3, 4},
				Digests:   []string{d2_pos, d2_pos, d3_neg, d3_neg},
				Labels:    []types.Label{pos, pos, neg, neg},
			},
			&LabeledTrace{
				CommitIds: []int{},
				Digests:   []string{},
				Labels:    []types.Label{},
			},
		},
		"test3": []*LabeledTrace{
			&LabeledTrace{
				CommitIds: []int{0, 4},
				Digests:   []string{d2_pos, d1_un},
				Labels:    []types.Label{pos, un},
			},
			&LabeledTrace{
				CommitIds: []int{1, 3},
				Digests:   []string{d2_pos, d1_un},
				Labels:    []types.Label{pos, un},
			},
			&LabeledTrace{
				CommitIds: []int{0, 4},
				Digests:   []string{d3_neg, d1_un},
				Labels:    []types.Label{neg, un},
			},
		},
	}

	test1_expectedBlameFreq := []int{2, 3, 0, 0, 0}
	test2_expectedBlameFreq := []int{2, 0, 0, 0, 0}
	test3_expectedBlameFreq := []int{2, 3, 3, 0}

	testTile_1 := &LabeledTile{
		Commits: commits,
		Traces:  traces,
	}

	blameLists := getBlameLists(testTile_1)
	assert.Equal(t, 3, len(blameLists.Blames))
	assert.NotNil(t, blameLists.Blames["test1"])
	assert.Equal(t, test1_expectedBlameFreq, blameLists.Blames["test1"][0].Freq)
	assert.Equal(t, test2_expectedBlameFreq, blameLists.Blames["test2"][0].Freq)
	assert.Equal(t, test3_expectedBlameFreq, blameLists.Blames["test3"][0].Freq)
}
