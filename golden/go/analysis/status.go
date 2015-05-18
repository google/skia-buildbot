package analysis

import (
	"go.skia.org/infra/golden/go/types"
	ptypes "go.skia.org/infra/perf/go/types"
)

// GUIStatus reflects the current rebaseline status. In particular whether
// HEAD is baselined and how many untriaged and negative digests there
// currently are.
type GUIStatus struct {
	// Indicates whether current HEAD is ok.
	OK bool `json:"ok"`

	// Last commit currently know.
	LastCommit *ptypes.Commit `json:"lastCommit"`

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
func calcStatus(state *AnalyzeState) *GUIStatus {
	corpStatus := make(map[string]*GUICorpusStatus, len(state.Index.corpora))
	minCommitId := map[string]int{}
	okByCorpus := map[string]bool{}

	// Gathers unique labels by corpus and label.
	byCorpus := map[string]map[types.Label]map[string]bool{}

	for _, corpus := range state.Index.corpora {
		minCommitId[corpus] = len(state.Tile.Commits)
		okByCorpus[corpus] = true
		byCorpus[corpus] = map[types.Label]map[string]bool{
			types.POSITIVE:  map[string]bool{},
			types.NEGATIVE:  map[string]bool{},
			types.UNTRIAGED: map[string]bool{},
		}
	}

	// Iterate over the current traces
	var idx int
	var corpus string
	for _, testTraces := range state.Tile.Traces {
		for _, trace := range testTraces {
			corpus = trace.Params[types.CORPUS_FIELD]
			idx = len(trace.Labels) - 1

			okByCorpus[corpus] = okByCorpus[corpus] && (trace.Labels[idx] == types.POSITIVE)
			if trace.CommitIds[idx] < minCommitId[corpus] {
				minCommitId[corpus] = trace.CommitIds[idx]
			}
			byCorpus[corpus][trace.Labels[idx]][trace.Digests[idx]] = true
		}
	}

	overallOk := true
	for _, corpus := range state.Index.corpora {
		overallOk = overallOk && okByCorpus[corpus]
		untriagedCount := len(byCorpus[corpus][types.UNTRIAGED])
		negativeCount := len(byCorpus[corpus][types.NEGATIVE])
		corpStatus[corpus] = &GUICorpusStatus{
			OK:             okByCorpus[corpus],
			MinCommitHash:  state.Tile.Commits[minCommitId[corpus]].Hash,
			UntriagedCount: untriagedCount,
			NegativeCount:  negativeCount,
		}
	}
	return &GUIStatus{
		OK:         overallOk,
		LastCommit: state.Tile.Commits[len(state.Tile.Commits)-1],
		CorpStatus: corpStatus,
	}
}
