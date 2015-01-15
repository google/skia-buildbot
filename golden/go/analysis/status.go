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

	// Status per corpus.
	CorpStatus map[string]*GUICorpusStatus `json:"corpStatus"`
}

type GUICorpusStatus struct {
	// Indicats whether this status is ok.
	OK bool `json:"ok"`

	// Earliest commit hash considered HEAD (is not always the last commit).
	MinCommitHash string `json:"minCommitHash"`

	// Number of untriaged digests in HEAD.
	UntriagedCount int `json:"untriagedCount"`

	// Number of negative digests in HEAD.
	NegativeCount int `json:"negativeCount"`
}

// calcStatus determines the status based on the current tile. It breaks
// down the status by individual corpora.
func (a *Analyzer) calcStatus(labeledTile *LabeledTile) *GUIStatus {
	corpStatus := make(map[string]*GUICorpusStatus, len(a.currentIndex.corpora))
	minCommitId := map[string]int{}
	okByCorpus := map[string]bool{}
	untriaged := map[string]map[string]bool{}
	negative := map[string]map[string]bool{}

	for _, corpus := range a.currentIndex.corpora {
		minCommitId[corpus] = len(labeledTile.Commits)
		okByCorpus[corpus] = true
		untriaged[corpus] = map[string]bool{}
		negative[corpus] = map[string]bool{}
	}

	// Iterate over the current traces
	var idx int
	var corpus string
	for _, testTraces := range labeledTile.Traces {
		for _, trace := range testTraces {
			corpus = trace.Params[types.CORPUS_FIELD]
			idx = len(trace.Labels) - 1

			okByCorpus[corpus] = okByCorpus[corpus] && (trace.Labels[idx] == types.POSITIVE)
			if trace.CommitIds[idx] < minCommitId[corpus] {
				minCommitId[corpus] = trace.CommitIds[idx]
			}
			if trace.Labels[idx] == types.UNTRIAGED {
				untriaged[corpus][trace.Digests[idx]] = true
			} else if trace.Labels[idx] == types.NEGATIVE {
				negative[corpus][trace.Digests[idx]] = true
			}
		}
	}

	overallOk := true
	for _, corpus := range a.currentIndex.corpora {
		overallOk = overallOk && okByCorpus[corpus]
		corpStatus[corpus] = &GUICorpusStatus{
			OK:             okByCorpus[corpus],
			MinCommitHash:  labeledTile.Commits[minCommitId[corpus]].Hash,
			UntriagedCount: len(untriaged[corpus]),
			NegativeCount:  len(negative[corpus]),
		}
	}

	return &GUIStatus{
		OK:         overallOk,
		CorpStatus: corpStatus,
	}
}
