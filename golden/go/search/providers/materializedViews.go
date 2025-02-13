package providers

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/sync/errgroup"
)

const (
	UnignoredRecentTracesView = "traces"
	ByBlameView               = "byblame"
)

// MaterializedViewProvider provides a struct for running materialized view related operations.
type MaterializedViewProvider struct {
	db                *pgxpool.Pool
	materializedViews map[string]bool
	windowLength      int

	// Metrics
	byBlameViewCounter     metrics2.Counter
	unignoredTracesCounter metrics2.Counter
}

// NewMaterializedViewProvider returns a new instance of the MaterializedViewProvider.
func NewMaterializedViewProvider(db *pgxpool.Pool, windowLength int) *MaterializedViewProvider {
	return &MaterializedViewProvider{
		db:           db,
		windowLength: windowLength,

		byBlameViewCounter:     metrics2.GetCounter("gold_mv_byblame_read"),
		unignoredTracesCounter: metrics2.GetCounter("gold_mv_unignored_read"),
	}
}

// StartMaterializedViews creates materialized views for non-ignored traces belonging to the
// given corpora. It starts a goroutine to keep these up to date.
func (s *MaterializedViewProvider) StartMaterializedViews(ctx context.Context, corpora []string, updateInterval time.Duration) error {
	_, span := trace.StartSpan(ctx, "StartMaterializedViews", trace.WithSampler(trace.AlwaysSample()))
	defer span.End()
	if len(corpora) == 0 {
		sklog.Infof("No materialized views configured")
		return nil
	}
	sklog.Infof("Initializing materialized views")
	eg, eCtx := errgroup.WithContext(ctx)
	var mutex sync.Mutex
	s.materializedViews = map[string]bool{}
	for _, c := range corpora {
		corpus := c
		eg.Go(func() error {
			mvName, err := s.createUnignoredRecentTracesView(eCtx, corpus)
			if err != nil {
				return skerr.Wrap(err)
			}
			mutex.Lock()
			defer mutex.Unlock()
			s.materializedViews[mvName] = true
			return nil
		})
		eg.Go(func() error {
			mvName, err := s.createByBlameView(eCtx, corpus)
			if err != nil {
				return skerr.Wrap(err)
			}
			mutex.Lock()
			defer mutex.Unlock()
			s.materializedViews[mvName] = true
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return skerr.Wrapf(err, "initializing materialized views %q", corpora)
	}

	sklog.Infof("Initialized %d materialized views", len(s.materializedViews))
	go util.RepeatCtx(ctx, updateInterval, func(ctx context.Context) {
		eg, eCtx := errgroup.WithContext(ctx)
		for v := range s.materializedViews {
			view := v
			eg.Go(func() error {
				statement := `REFRESH MATERIALIZED VIEW ` + view
				_, err := s.db.Exec(eCtx, statement)
				return skerr.Wrapf(err, "updating %s", view)
			})
		}
		if err := eg.Wait(); err != nil {
			sklog.Warningf("Could not refresh material views: %s", err)
		}
	})
	return nil
}

func (s *MaterializedViewProvider) createUnignoredRecentTracesView(ctx context.Context, corpus string) (string, error) {
	mvName := "mv_" + corpus + "_" + UnignoredRecentTracesView
	statement := "CREATE MATERIALIZED VIEW IF NOT EXISTS " + mvName
	statement += `
AS WITH
BeginningOfWindow AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC
	OFFSET ` + strconv.Itoa(s.windowLength-1) + ` LIMIT 1
)
SELECT trace_id, grouping_id, digest FROM ValuesAtHead
JOIN BeginningOfWindow ON ValuesAtHead.most_recent_commit_id >= BeginningOfWindow.commit_id
WHERE corpus = '` + corpus + `' AND matches_any_ignore_rule = FALSE
`
	_, err := s.db.Exec(ctx, statement)
	return mvName, skerr.Wrap(err)
}

func (s *MaterializedViewProvider) createByBlameView(ctx context.Context, corpus string) (string, error) {
	mvName := "mv_" + corpus + "_" + ByBlameView
	statement := "CREATE MATERIALIZED VIEW IF NOT EXISTS " + mvName
	statement += `
AS WITH
BeginningOfWindow AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC
	OFFSET ` + strconv.Itoa(s.windowLength-1) + ` LIMIT 1
),
UntriagedDigests AS (
	SELECT grouping_id, digest FROM Expectations
	WHERE label = 'u'
),
UnignoredDataAtHead AS (
	SELECT trace_id, grouping_id, digest FROM ValuesAtHead
	JOIN BeginningOfWindow ON ValuesAtHead.most_recent_commit_id >= BeginningOfWindow.commit_id
	WHERE matches_any_ignore_rule = FALSE AND corpus = '` + corpus + `'
)
SELECT UnignoredDataAtHead.trace_id, UnignoredDataAtHead.grouping_id, UnignoredDataAtHead.digest FROM
UntriagedDigests
JOIN UnignoredDataAtHead ON UntriagedDigests.grouping_id = UnignoredDataAtHead.grouping_id AND
	 UntriagedDigests.digest = UnignoredDataAtHead.digest
`
	_, err := s.db.Exec(ctx, statement)
	return mvName, skerr.Wrap(err)
}

// GetMaterializedView returns the name of the materialized view if it has been created, or empty
// string if there is not such a view.
func (s *MaterializedViewProvider) GetMaterializedView(viewName, corpus string) string {
	if viewName == ByBlameView {
		s.byBlameViewCounter.Inc(1)
	}
	if viewName == UnignoredRecentTracesView {
		s.unignoredTracesCounter.Inc(1)
	}
	if len(s.materializedViews) == 0 {
		return ""
	}
	mv := "mv_" + corpus + "_" + viewName
	if s.materializedViews[mv] {
		return mv
	}
	return ""
}
