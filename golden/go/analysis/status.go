package analysis

import (
	"fmt"

	"github.com/rcrowley/go-metrics"
	"go.skia.org/infra/golden/go/types"
	ptypes "go.skia.org/infra/perf/go/types"
)

const (
	// Metric names and templates for metric names added in this file.
	METRIC_TOTAL       = "gold.digests.total"
	METRIC_ALL_TMPL    = "gold.%s.all"
	METRIC_CORPUS_TMPL = "gold.%s.by_corpus.%s"
)

var (
	// Gauges to track overall digests with different labels.
	allUntriagedGauge = metrics.NewRegisteredGauge(fmt.Sprintf(METRIC_ALL_TMPL, types.UNTRIAGED), nil)
	allPositiveGauge  = metrics.NewRegisteredGauge(fmt.Sprintf(METRIC_ALL_TMPL, types.POSITIVE), nil)
	allNegativeGauge  = metrics.NewRegisteredGauge(fmt.Sprintf(METRIC_ALL_TMPL, types.NEGATIVE), nil)
	totalGauge        = metrics.NewRegisteredGauge(METRIC_TOTAL, nil)

	// Gauges to track counts of digests by corpus / label
	corpusGauges = map[string]map[types.Label]metrics.Gauge{}
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
		if _, ok := corpusGauges[corpus]; !ok {
			corpusGauges[corpus] = map[types.Label]metrics.Gauge{
				types.UNTRIAGED: metrics.NewRegisteredGauge(fmt.Sprintf(METRIC_CORPUS_TMPL, types.UNTRIAGED, corpus), nil),
				types.POSITIVE:  metrics.NewRegisteredGauge(fmt.Sprintf(METRIC_CORPUS_TMPL, types.POSITIVE, corpus), nil),
				types.NEGATIVE:  metrics.NewRegisteredGauge(fmt.Sprintf(METRIC_CORPUS_TMPL, types.NEGATIVE, corpus), nil),
			}
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
	allUntriagedCount := 0
	allPositiveCount := 0
	allNegativeCount := 0
	for _, corpus := range state.Index.corpora {
		overallOk = overallOk && okByCorpus[corpus]
		untriagedCount := len(byCorpus[corpus][types.UNTRIAGED])
		positiveCount := len(byCorpus[corpus][types.POSITIVE])
		negativeCount := len(byCorpus[corpus][types.NEGATIVE])
		corpStatus[corpus] = &GUICorpusStatus{
			OK:             okByCorpus[corpus],
			MinCommitHash:  state.Tile.Commits[minCommitId[corpus]].Hash,
			UntriagedCount: untriagedCount,
			NegativeCount:  negativeCount,
		}
		allUntriagedCount += untriagedCount
		allNegativeCount += negativeCount
		allPositiveCount += positiveCount

		corpusGauges[corpus][types.POSITIVE].Update(int64(positiveCount))
		corpusGauges[corpus][types.NEGATIVE].Update(int64(negativeCount))
		corpusGauges[corpus][types.UNTRIAGED].Update(int64(untriagedCount))
	}
	allUntriagedGauge.Update(int64(allUntriagedCount))
	allPositiveGauge.Update(int64(allPositiveCount))
	allNegativeGauge.Update(int64(allNegativeCount))
	totalGauge.Update(int64(allUntriagedCount + allPositiveCount + allNegativeCount))

	return &GUIStatus{
		OK:         overallOk,
		LastCommit: state.Tile.Commits[len(state.Tile.Commits)-1],
		CorpStatus: corpStatus,
	}
}
