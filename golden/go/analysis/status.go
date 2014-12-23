package analysis

import (
	"skia.googlesource.com/buildbot.git/golden/go/types"
)

// GUIStatus reflects the current rebaseline status. In particular whether
// HEAD is baselined and how many untriaged and negative digests there
// currently are.
type GUIStatus struct {
	// Indicates whether current HEAD is ok.
	OK bool `json:"ok"`

	// Earliest commit hash considered HEAD (is not always the last commit).
	MinCommitHash string `json:"minCommitHash"`

	// Number of untriaged digests in HEAD.
	UntriagedCount int `json:"untriagedCount"`

	// Number of negative digests in HEAD.
	NegativeCount int `json:"negativeCount"`
}

func calcStatus(labeledTile *LabeledTile) *GUIStatus {
	// Iterate over the current traces
	minCommitId := len(labeledTile.Commits)
	ok := true
	untriagedSet := map[string]bool{}
	negativeSet := map[string]bool{}
	var idx int
	for _, testTraces := range labeledTile.Traces {
		for _, trace := range testTraces {
			idx = len(trace.Labels) - 1
			ok = ok && (trace.Labels[idx] == types.POSITIVE)
			if trace.CommitIds[idx] < minCommitId {
				minCommitId = trace.CommitIds[idx]
			}
			if trace.Labels[idx] == types.UNTRIAGED {
				untriagedSet[trace.Digests[idx]] = true
			} else if trace.Labels[idx] == types.NEGATIVE {
				negativeSet[trace.Digests[idx]] = true
			}
		}
	}

	return &GUIStatus{
		OK:             ok,
		MinCommitHash:  labeledTile.Commits[minCommitId].Hash,
		UntriagedCount: len(untriagedSet),
		NegativeCount:  len(negativeSet),
	}
}
