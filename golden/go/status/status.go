package status

import (
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Metric names and templates for metric names added in this file.
	METRIC_TOTAL  = "gold.status.total-digests"
	METRIC_ALL    = "gold.status.all"
	METRIC_CORPUS = "gold.status.by-corpus"
)

// GUIStatus reflects the current rebaseline status. In particular whether
// HEAD is baselined and how many untriaged and negative digests there
// currently are.
type GUIStatus struct {
	// Indicates whether current HEAD is ok.
	OK bool `json:"ok"`

	FirstCommit *tiling.Commit `json:"firstCommit"`

	// Last commit currently know.
	LastCommit *tiling.Commit `json:"lastCommit"`

	TotalCommits  int `json:"totalCommits"`
	FilledCommits int `json:"filledCommits"`

	// Status per corpus.
	CorpStatus []*GUICorpusStatus `json:"corpStatus"`
}

type GUICorpusStatus struct {
	// Name of the corpus.
	Name string `json:"name"`

	// Indicates whether this status is ok.
	OK bool `json:"ok"`

	// Earliest commit hash considered HEAD (is not always the last commit).
	MinCommitHash string `json:"minCommitHash"`

	// Number of untriaged digests in HEAD.
	UntriagedCount int `json:"untriagedCount"`

	// Number of negative digests in HEAD.
	NegativeCount int `json:"negativeCount"`
}

type CorpusStatusSorter []*GUICorpusStatus

// Implement sort.Interface
func (c CorpusStatusSorter) Len() int           { return len(c) }
func (c CorpusStatusSorter) Less(i, j int) bool { return c[i].Name < c[j].Name }
func (c CorpusStatusSorter) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }

type StatusWatcher struct {
	storages *storage.Storage
	current  *GUIStatus
	mutex    sync.Mutex

	// Gauges to track overall digests with different labels.
	allUntriagedGauge metrics2.Int64Metric
	allPositiveGauge  metrics2.Int64Metric
	allNegativeGauge  metrics2.Int64Metric
	totalGauge        metrics2.Int64Metric

	// Gauges to track counts of digests by corpus / label
	corpusGauges map[string]map[types.Label]metrics2.Int64Metric
}

func New(storages *storage.Storage) (*StatusWatcher, error) {
	ret := &StatusWatcher{
		storages:          storages,
		allUntriagedGauge: metrics2.GetInt64Metric(METRIC_ALL, map[string]string{"type": types.UNTRIAGED.String()}),
		allPositiveGauge:  metrics2.GetInt64Metric(METRIC_ALL, map[string]string{"type": types.POSITIVE.String()}),
		allNegativeGauge:  metrics2.GetInt64Metric(METRIC_ALL, map[string]string{"type": types.NEGATIVE.String()}),
		totalGauge:        metrics2.GetInt64Metric(METRIC_TOTAL, nil),
		corpusGauges:      map[string]map[types.Label]metrics2.Int64Metric{},
	}

	if err := ret.calcAndWatchStatus(); err != nil {
		return nil, err
	}

	return ret, nil
}

func (s *StatusWatcher) GetStatus() *GUIStatus {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.current
}

func (s *StatusWatcher) calcAndWatchStatus() error {
	sklog.Infof("Starting status watcher")
	expChanges := make(chan types.TestExp)
	s.storages.EventBus.SubscribeAsync(expstorage.EV_EXPSTORAGE_CHANGED, func(e interface{}) {
		expChanges <- e.(*expstorage.EventExpectationChange).TestChanges
	})

	tileStream := s.storages.GetTileStreamNow(2 * time.Minute)
	sklog.Infof("Got tile stream for status watcher")

	lastCpxTile := <-tileStream
	sklog.Infof("Received first tile for status watcher")

	if err := s.calcStatus(lastCpxTile); err != nil {
		return err
	}
	sklog.Infof("Calculated first status")

	liveness := metrics2.NewLiveness("gold_status_monitoring")

	go func() {
		for {
			select {
			case <-tileStream:
				cpxTile, err := s.storages.GetLastTileTrimmed()
				if err != nil {
					sklog.Errorf("Error retrieving tile: %s", err)
					continue
				}

				if err := s.calcStatus(cpxTile); err != nil {
					sklog.Errorf("Error calculating status: %s", err)
				} else {
					lastCpxTile = cpxTile
					liveness.Reset()
				}
			case <-expChanges:
				storage.DrainChangeChannel(expChanges)
				if err := s.calcStatus(lastCpxTile); err != nil {
					sklog.Errorf("Error calculating tile after expectation update: %s", err)
				}
				liveness.Reset()
			}
		}
	}()
	sklog.Infof("Done starting status watcher")

	return nil
}

func (s *StatusWatcher) calcStatus(cpxTile *types.ComplexTile) error {
	defer timer.New("Calc status timer:").Stop()

	okByCorpus := map[string]bool{}
	expectations, err := s.storages.ExpectationsStore.Get()
	if err != nil {
		return err
	}

	// Gathers unique labels by corpus and label.
	byCorpus := map[string]map[types.Label]map[string]bool{}

	// Iterate over the current traces
	dataTile := cpxTile.GetTile(false)
	tileLen := dataTile.LastCommitIndex() + 1
	for _, trace := range dataTile.Traces {
		gTrace := trace.(*types.GoldenTrace)

		idx := tileLen - 1
		for (idx >= 0) && (gTrace.Digests[idx] == types.MISSING_DIGEST) {
			idx--
		}

		// If this is an empty trace we ignore it for now.
		if idx == -1 {
			continue
		}

		// If this corpus doesn't exist yet, we initialize it.
		corpus := gTrace.Params()[types.CORPUS_FIELD]
		if _, ok := byCorpus[corpus]; !ok {
			okByCorpus[corpus] = true
			byCorpus[corpus] = map[types.Label]map[string]bool{
				types.POSITIVE:  {},
				types.NEGATIVE:  {},
				types.UNTRIAGED: {},
			}

			if _, ok := s.corpusGauges[corpus]; !ok {
				s.corpusGauges[corpus] = map[types.Label]metrics2.Int64Metric{
					types.UNTRIAGED: metrics2.GetInt64Metric(METRIC_CORPUS, map[string]string{"type": types.UNTRIAGED.String(), "corpus": corpus}),
					types.POSITIVE:  metrics2.GetInt64Metric(METRIC_CORPUS, map[string]string{"type": types.POSITIVE.String(), "corpus": corpus}),
					types.NEGATIVE:  metrics2.GetInt64Metric(METRIC_CORPUS, map[string]string{"type": types.NEGATIVE.String(), "corpus": corpus}),
				}
			}
		}

		// Account for the corpus and testname.
		digest := gTrace.Digests[idx]
		testName := gTrace.Params()[types.PRIMARY_KEY_FIELD]
		status := expectations.Classification(testName, digest)

		okByCorpus[corpus] = okByCorpus[corpus] &&
			((status == types.POSITIVE) || (status == types.NEGATIVE))
		byCorpus[corpus][status][testName+digest] = true
	}

	overallOk := true
	allUntriagedCount := 0
	allPositiveCount := 0
	allNegativeCount := 0
	corpStatus := make([]*GUICorpusStatus, 0, len(byCorpus))
	for corpus := range byCorpus {
		overallOk = overallOk && okByCorpus[corpus]
		untriagedCount := len(byCorpus[corpus][types.UNTRIAGED])
		positiveCount := len(byCorpus[corpus][types.POSITIVE])
		negativeCount := len(byCorpus[corpus][types.NEGATIVE])
		corpStatus = append(corpStatus, &GUICorpusStatus{
			Name:           corpus,
			OK:             okByCorpus[corpus],
			UntriagedCount: untriagedCount,
			NegativeCount:  negativeCount,
		})
		allUntriagedCount += untriagedCount
		allNegativeCount += negativeCount
		allPositiveCount += positiveCount

		s.corpusGauges[corpus][types.POSITIVE].Update(int64(positiveCount))
		s.corpusGauges[corpus][types.NEGATIVE].Update(int64(negativeCount))
		s.corpusGauges[corpus][types.UNTRIAGED].Update(int64(untriagedCount))
	}
	s.allUntriagedGauge.Update(int64(allUntriagedCount))
	s.allPositiveGauge.Update(int64(allPositiveCount))
	s.allNegativeGauge.Update(int64(allNegativeCount))
	s.totalGauge.Update(int64(allUntriagedCount + allPositiveCount + allNegativeCount))

	sort.Sort(CorpusStatusSorter(corpStatus))

	allCommits := cpxTile.AllCommits()
	result := &GUIStatus{
		OK:            overallOk,
		FirstCommit:   allCommits[0],
		LastCommit:    allCommits[len(allCommits)-1],
		TotalCommits:  len(allCommits),
		FilledCommits: cpxTile.FilledCommits(),
		CorpStatus:    corpStatus,
	}

	// Swap out the current tile.
	s.mutex.Lock()
	s.current = result
	s.mutex.Unlock()

	return nil
}
