package status

import (
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/tilesource"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Metric names and templates for metric names added in this file.
	METRIC_TOTAL  = "gold_status_total_digests"
	METRIC_ALL    = "gold_status_all"
	METRIC_CORPUS = "gold_status_by_corpus"
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

type StatusWatcherConfig struct {
	EventBus          eventbus.EventBus
	ExpectationsStore expstorage.ExpectationsStore
	TileSource        tilesource.TileSource
	VCS               vcsinfo.VCS
}

type StatusWatcher struct {
	StatusWatcherConfig
	current *GUIStatus
	mutex   sync.Mutex

	// Gauges to track overall digests with different labels.
	allUntriagedGauge metrics2.Int64Metric
	allPositiveGauge  metrics2.Int64Metric
	allNegativeGauge  metrics2.Int64Metric
	totalGauge        metrics2.Int64Metric

	// Gauges to track counts of digests by corpus / label
	corpusGauges map[string]map[types.Label]metrics2.Int64Metric
}

func New(swc StatusWatcherConfig) (*StatusWatcher, error) {
	ret := &StatusWatcher{
		StatusWatcherConfig: swc,
		allUntriagedGauge:   metrics2.GetInt64Metric(METRIC_ALL, map[string]string{"type": types.UNTRIAGED.String()}),
		allPositiveGauge:    metrics2.GetInt64Metric(METRIC_ALL, map[string]string{"type": types.POSITIVE.String()}),
		allNegativeGauge:    metrics2.GetInt64Metric(METRIC_ALL, map[string]string{"type": types.NEGATIVE.String()}),
		totalGauge:          metrics2.GetInt64Metric(METRIC_TOTAL, nil),
		corpusGauges:        map[string]map[types.Label]metrics2.Int64Metric{},
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

// updateLastCommitAge calculates how old the last commit in Gold is and reports
// it to metrics, so we can alert on it. It computes the age in two ways - absolute
// age (wall_time) and time since a newer commit landed (with_new_commit). The latter
// is the preferred metric due to the lower false-positive chance, with the former
// being a good backup since it has a lower false-negative chance.
// updateLastCommitAge is thread-safe.
func (s *StatusWatcher) updateLastCommitAge() {
	st := s.GetStatus()
	if st == nil {
		sklog.Warningf("GetStatus() was nil when computing metrics")
		return
	}
	if st.LastCommit == nil {
		sklog.Warningf("GetStatus() had nil LastCommit when computing metrics: %#v", st)
		return
	}

	lastCommitAge := metrics2.GetInt64Metric("gold_last_commit_age_s", map[string]string{
		"type": "wall_time",
	})
	lastCommitUnix := st.LastCommit.CommitTime // already in seconds since epoch
	lastCommitAge.Update(time.Now().Unix() - lastCommitUnix)

	if s.VCS == nil {
		sklog.Warningf("skipping updateLastCommitAge because VCS not set up")
		return
	}
	// Start looking one second after the commit we know about to avoid erroneously
	// alerting when two commits land in the same second.
	commitsFromLast := s.VCS.Range(time.Unix(st.LastCommit.CommitTime+1, 0), time.Now())
	uningestedCommitAgeMetric := metrics2.GetInt64Metric("gold_last_commit_age_s", map[string]string{
		"type": "with_new_commit",
	})
	if len(commitsFromLast) == 0 {
		uningestedCommitAgeMetric.Update(0)
	} else {
		uningestedCommitAgeMetric.Update(time.Now().Unix() - commitsFromLast[1].Timestamp.Unix())
	}
}

func (s *StatusWatcher) calcAndWatchStatus() error {
	sklog.Infof("Starting status watcher")
	expChanges := make(chan types.Expectations)
	s.EventBus.SubscribeAsync(expstorage.EV_EXPSTORAGE_CHANGED, func(e interface{}) {
		expChanges <- e.(*expstorage.EventExpectationChange).TestChanges
	})

	tileStream := tilesource.GetTileStreamNow(s.TileSource, 2*time.Minute, "status-watcher")
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
				cpxTile, err := s.TileSource.GetTile()
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
				drainChangeChannel(expChanges)
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

func (s *StatusWatcher) calcStatus(cpxTile types.ComplexTile) error {
	defer s.updateLastCommitAge()
	defer shared.NewMetricsTimer("calculate_status").Stop()

	okByCorpus := map[string]bool{}
	expectations, err := s.ExpectationsStore.Get()
	if err != nil {
		return err
	}

	// Gathers unique labels by corpus and label.
	byCorpus := map[string]map[types.Label]map[string]bool{}

	// Iterate over the current traces
	dataTile := cpxTile.GetTile(types.ExcludeIgnoredTraces)
	if len(dataTile.Commits) == 0 {
		sklog.Warningf("Empty tile, doing nothing")
		return nil
	}
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
		corpus := gTrace.Corpus()
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
		testName := gTrace.TestName()
		status := expectations.Classification(testName, digest)

		okByCorpus[corpus] = okByCorpus[corpus] &&
			((status == types.POSITIVE) || (status == types.NEGATIVE))
		byCorpus[corpus][status][string(testName)+string(digest)] = true
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
	defer s.mutex.Unlock()
	s.current = result

	return nil
}

// drainChangeChannel removes everything from the channel that's currently
// buffered or ready to be read.
func drainChangeChannel(ch <-chan types.Expectations) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
