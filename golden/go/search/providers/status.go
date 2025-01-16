package providers

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/web/frontend"
)

// StatusProvider provides a struct for retrieving status information on the corpora for the instance.
type StatusProvider struct {
	db           *pgxpool.Pool
	windowLength int
	mutex        sync.RWMutex
	// This caches the corpora names that are publicly visible.
	publiclyVisibleCorpora map[string]struct{}
	// This caches the trace ids that are publicly visible.
	publiclyVisibleTraces map[schema.MD5Hash]struct{}
}

// NewStatusProvider returns a new instance of the StatusProvider.
func NewStatusProvider(db *pgxpool.Pool, windowLength int) *StatusProvider {
	return &StatusProvider{
		db:           db,
		windowLength: windowLength,
	}
}

// SetPublicCorpora sets the given corpora as the publicly visble ones.
func (s *StatusProvider) SetPublicCorpora(corpora map[string]struct{}) {
	s.publiclyVisibleCorpora = corpora
}

// SetPublicTraces sets the given traces as the publicly visible ones.
func (s *StatusProvider) SetPublicTraces(traces map[schema.MD5Hash]struct{}) {
	s.publiclyVisibleTraces = traces
}

// GetStatusForAllCorpora returns the status information for all corpora.
func (s *StatusProvider) GetStatusForAllCorpora(ctx context.Context, isPublic bool) (frontend.Commit, []frontend.GUICorpusStatus, error) {
	var commit frontend.Commit
	row := s.db.QueryRow(ctx, `SELECT commit_id FROM CommitsWithData
    ORDER BY CommitsWithData.commit_id DESC LIMIT 1`)
	if err := row.Scan(&commit.ID); err != nil {
		return commit, nil, skerr.Wrap(err)
	}

	row = s.db.QueryRow(ctx, `SELECT git_hash, commit_time, author_email, subject
    FROM GitCommits WHERE GitCommits.commit_id = $1`, commit.ID)
	var ts time.Time
	if err := row.Scan(&commit.Hash, &ts, &commit.Author, &commit.Subject); err != nil {
		sklog.Infof("Error getting git info for commit_id %s - %s", commit.ID, err)
	} else {
		commit.CommitTime = ts.UTC().Unix()
	}

	if isPublic {
		xcs, err := s.getPublicViewCorporaStatuses(ctx)
		if err != nil {
			return commit, nil, skerr.Wrap(err)
		}

		return commit, xcs, nil
	}

	xcs, err := s.getCorporaStatuses(ctx)
	if err != nil {
		return commit, nil, skerr.Wrap(err)
	}

	return commit, xcs, nil
}

// getCorporaStatuses counts the untriaged digests for all corpora.
func (s *StatusProvider) getCorporaStatuses(ctx context.Context) ([]frontend.GUICorpusStatus, error) {
	ctx, span := trace.StartSpan(ctx, "getCorporaStatuses")
	defer span.End()
	const statement = `WITH
CommitsInWindow AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
),
OldestCommitInWindow AS (
	SELECT commit_id FROM CommitsInWindow
	ORDER BY commit_id ASC LIMIT 1
),
DistinctNotIgnoredDigests AS (
	SELECT DISTINCT corpus, digest, grouping_id FROM ValuesAtHead
	JOIN OldestCommitInWindow ON ValuesAtHead.most_recent_commit_id >= OldestCommitInWindow.commit_id
	WHERE matches_any_ignore_rule = FALSE
),
CorporaWithAtLeastOneTriaged AS (
    SELECT corpus, COUNT(DistinctNotIgnoredDigests.digest) AS num_untriaged FROM DistinctNotIgnoredDigests
    JOIN Expectations ON DistinctNotIgnoredDigests.grouping_id = Expectations.grouping_id AND
        DistinctNotIgnoredDigests.digest = Expectations.digest AND label = 'u'
    GROUP BY corpus
),
AllCorpora AS (
    -- Corpora with no untriaged digests will not show up in CorporaWithAtLeastOneTriaged.
    -- We still want to include them in our status, so we do a separate query and union it in.
    SELECT DISTINCT corpus, 0 AS num_untriaged FROM ValuesAtHead
    JOIN OldestCommitInWindow ON ValuesAtHead.most_recent_commit_id >= OldestCommitInWindow.commit_id
)
SELECT corpus, max(num_untriaged) FROM (
    SELECT corpus, num_untriaged FROM AllCorpora
    UNION
    SELECT corpus, num_untriaged FROM CorporaWithAtLeastOneTriaged
) as all_corpora GROUP BY corpus`

	rows, err := s.db.Query(ctx, statement, s.windowLength)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []frontend.GUICorpusStatus
	for rows.Next() {
		var cs frontend.GUICorpusStatus
		if err := rows.Scan(&cs.Name, &cs.UntriagedCount); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, cs)
	}

	sort.Slice(rv, func(i, j int) bool {
		return rv[i].Name < rv[j].Name
	})
	return rv, nil
}

// getPublicViewCorporaStatuses counts the untriaged digests belonging to only those traces which
// match the public view matcher. It filters the traces using the cached publiclyVisibleTraces.
func (s *StatusProvider) getPublicViewCorporaStatuses(ctx context.Context) ([]frontend.GUICorpusStatus, error) {
	ctx, span := trace.StartSpan(ctx, "getCorporaStatuses")
	defer span.End()
	const statement = `WITH
CommitsInWindow AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
),
OldestCommitInWindow AS (
	SELECT commit_id FROM CommitsInWindow
	ORDER BY commit_id ASC LIMIT 1
),
NotIgnoredDigests AS (
	SELECT trace_id, corpus, digest, grouping_id FROM ValuesAtHead
	JOIN OldestCommitInWindow ON ValuesAtHead.most_recent_commit_id >= OldestCommitInWindow.commit_id
	WHERE matches_any_ignore_rule = FALSE AND corpus = ANY($2)
)
SELECT trace_id, corpus FROM NotIgnoredDigests
JOIN Expectations ON NotIgnoredDigests.grouping_id = Expectations.grouping_id AND
	NotIgnoredDigests.digest = Expectations.digest AND label = 'u'
`

	s.mutex.RLock()
	defer s.mutex.RUnlock()
	corpusCount := map[string]int{}
	var corporaArgs []string
	for corpus := range s.publiclyVisibleCorpora {
		corpusCount[corpus] = 0 // make sure we include all corpora, even those with 0 untriaged.
		corporaArgs = append(corporaArgs, corpus)
	}

	rows, err := s.db.Query(ctx, statement, s.windowLength, corporaArgs)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()

	var traceKey schema.MD5Hash
	for rows.Next() {
		var tr schema.TraceID
		var corpus string
		if err := rows.Scan(&tr, &corpus); err != nil {
			return nil, skerr.Wrap(err)
		}
		copy(traceKey[:], tr)
		if _, ok := s.publiclyVisibleTraces[traceKey]; ok {
			corpusCount[corpus]++
		}
	}

	var rv []frontend.GUICorpusStatus
	for corpus, count := range corpusCount {
		rv = append(rv, frontend.GUICorpusStatus{
			Name:           corpus,
			UntriagedCount: count,
		})
	}

	sort.Slice(rv, func(i, j int) bool {
		return rv[i].Name < rv[j].Name
	})

	return rv, nil
}
