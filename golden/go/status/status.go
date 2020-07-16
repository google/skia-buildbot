package status

import (
	"context"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/tilesource"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

const (
	// Metric names and templates for metric names added in this file.
	totalDigestsMetric = "gold_status_total_digests"
	allMetric          = "gold_status_all"
	corpusMetric       = "gold_status_by_corpus"
)

// GUIStatus reflects the current rebaseline status. In particular whether
// HEAD is baselined and how many untriaged and negative digests there
// currently are.
type GUIStatus struct {
	// Indicates whether current HEAD is ok.
	OK bool `json:"ok"`

	FirstCommit frontend.Commit `json:"firstCommit"`

	// Last commit currently know.
	LastCommit frontend.Commit `json:"lastCommit"`

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
	ChangeListener    expectations.ChangeEventRegisterer
	ExpectationsStore expectations.Store
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
	corpusGauges map[string]map[expectations.Label]metrics2.Int64Metric
}

func New(ctx context.Context, swc StatusWatcherConfig) (*StatusWatcher, error) {
	ret := &StatusWatcher{
		StatusWatcherConfig: swc,
		allUntriagedGauge:   metrics2.GetInt64Metric(allMetric, map[string]string{"type": string(expectations.UntriagedStr)}),
		allPositiveGauge:    metrics2.GetInt64Metric(allMetric, map[string]string{"type": string(expectations.PositiveStr)}),
		allNegativeGauge:    metrics2.GetInt64Metric(allMetric, map[string]string{"type": string(expectations.NegativeStr)}),
		totalGauge:          metrics2.GetInt64Metric(totalDigestsMetric, nil),
		corpusGauges:        map[string]map[expectations.Label]metrics2.Int64Metric{},
	}

	if err := ret.calcAndWatchStatus(ctx); err != nil {
		return nil, skerr.Wrap(err)
	}

	return ret, nil
}

// GetStatus returns the current status, ready to be sent to the frontend.
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
	if st.LastCommit.CommitTime == 0 {
		sklog.Warningf("GetStatus() had empty LastCommit when computing metrics: %#v", st)
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
		oldestNoningestedCommit := commitsFromLast[0]
		uningestedCommitAgeMetric.Update(time.Now().Unix() - oldestNoningestedCommit.Timestamp.Unix())
	}
}

func (s *StatusWatcher) calcAndWatchStatus(ctx context.Context) error {
	sklog.Infof("Starting status watcher")
	// This value chosen arbitrarily in an effort to avoid blockages on this channel.
	expChanges := make(chan expectations.ID, 1000)
	s.ChangeListener.ListenForChange(func(e expectations.ID) {
		expChanges <- e
	})

	tileStream := tilesource.GetTileStreamNow(s.TileSource, 2*time.Minute, "status-watcher")
	sklog.Infof("Got tile stream for status watcher")

	lastCpxTile := <-tileStream
	sklog.Infof("Received first tile for status watcher")

	if err := s.calcStatus(ctx, lastCpxTile); err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("Calculated first status")

	liveness := metrics2.NewLiveness("gold_status_monitoring")
	go func() {
		for {
			select {
			case cpxTile := <-tileStream:
				if err := s.calcStatus(ctx, cpxTile); err != nil {
					sklog.Errorf("Error calculating status: %s", err)
				} else {
					lastCpxTile = cpxTile
					liveness.Reset()
				}
			case <-expChanges:
				drainChangeChannel(expChanges)
				if err := s.calcStatus(ctx, lastCpxTile); err != nil {
					sklog.Errorf("Error calculating tile after expectation update: %s", err)
				} else {
					liveness.Reset()
				}
			}
		}
	}()
	sklog.Infof("Done starting status watcher")

	return nil
}

func (s *StatusWatcher) calcStatus(ctx context.Context, cpxTile tiling.ComplexTile) error {
	defer s.updateLastCommitAge()
	defer shared.NewMetricsTimer("calculate_status").Stop()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	okByCorpus := map[string]bool{}
	exp, err := s.ExpectationsStore.Get(ctx)
	if err != nil {
		return skerr.Wrapf(err, "fetching expectations")
	}

	// Gathers unique labels by corpus and label.
	byCorpus := map[string]map[expectations.Label]map[string]bool{}

	// Iterate over the current traces
	dataTile := cpxTile.GetTile(types.ExcludeIgnoredTraces)
	if len(dataTile.Commits) == 0 {
		sklog.Warningf("Empty tile, doing nothing")
		return nil
	}
	tileLen := dataTile.LastCommitIndex() + 1
	for _, trace := range dataTile.Traces {
		if err := ctx.Err(); err != nil {
			return skerr.Wrap(err)
		}

		idx := tileLen - 1
		for (idx >= 0) && (trace.Digests[idx] == tiling.MissingDigest) {
			idx--
		}

		// If this is an empty trace we ignore it for now.
		if idx == -1 {
			continue
		}

		// If this corpus doesn't exist yet, we initialize it.
		corpus := trace.Corpus()
		if _, ok := byCorpus[corpus]; !ok {
			okByCorpus[corpus] = true
			byCorpus[corpus] = map[expectations.Label]map[string]bool{
				expectations.Positive:  {},
				expectations.Negative:  {},
				expectations.Untriaged: {},
			}

			if _, ok := s.corpusGauges[corpus]; !ok {
				s.corpusGauges[corpus] = map[expectations.Label]metrics2.Int64Metric{
					expectations.Untriaged: metrics2.GetInt64Metric(corpusMetric, map[string]string{"type": string(expectations.UntriagedStr), "corpus": corpus}),
					expectations.Positive:  metrics2.GetInt64Metric(corpusMetric, map[string]string{"type": string(expectations.PositiveStr), "corpus": corpus}),
					expectations.Negative:  metrics2.GetInt64Metric(corpusMetric, map[string]string{"type": string(expectations.NegativeStr), "corpus": corpus}),
				}
			}
		}

		// Account for the corpus and testname.
		digest := trace.Digests[idx]
		testName := trace.TestName()
		status := exp.Classification(testName, digest)

		okByCorpus[corpus] = okByCorpus[corpus] &&
			((status == expectations.Positive) || (status == expectations.Negative))
		byCorpus[corpus][status][string(testName)+string(digest)] = true
	}

	overallOk := true
	allUntriagedCount := 0
	allPositiveCount := 0
	allNegativeCount := 0
	corpStatus := make([]*GUICorpusStatus, 0, len(byCorpus))
	for corpus := range byCorpus {
		overallOk = overallOk && okByCorpus[corpus]
		untriagedCount := len(byCorpus[corpus][expectations.Untriaged])
		positiveCount := len(byCorpus[corpus][expectations.Positive])
		negativeCount := len(byCorpus[corpus][expectations.Negative])
		corpStatus = append(corpStatus, &GUICorpusStatus{
			Name:           corpus,
			OK:             okByCorpus[corpus],
			UntriagedCount: untriagedCount,
			NegativeCount:  negativeCount,
		})
		allUntriagedCount += untriagedCount
		allNegativeCount += negativeCount
		allPositiveCount += positiveCount

		s.corpusGauges[corpus][expectations.Positive].Update(int64(positiveCount))
		s.corpusGauges[corpus][expectations.Negative].Update(int64(negativeCount))
		s.corpusGauges[corpus][expectations.Untriaged].Update(int64(untriagedCount))
	}
	s.allUntriagedGauge.Update(int64(allUntriagedCount))
	s.allPositiveGauge.Update(int64(allPositiveCount))
	s.allNegativeGauge.Update(int64(allNegativeCount))
	s.totalGauge.Update(int64(allUntriagedCount + allPositiveCount + allNegativeCount))

	sort.Sort(CorpusStatusSorter(corpStatus))
	allCommits := cpxTile.AllCommits()
	result := &GUIStatus{
		OK:            overallOk,
		FirstCommit:   frontend.FromTilingCommit(allCommits[0]),
		LastCommit:    frontend.FromTilingCommit(allCommits[len(allCommits)-1]),
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
func drainChangeChannel(ch <-chan expectations.ID) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
