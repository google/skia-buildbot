package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/cache"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/web/frontend"
)

const (
	commitCacheSize          = 5_000
	optionsGroupingCacheSize = 50_000
)

// CommitsProvider provides a struct for retrieving commit related data.
type CommitsProvider struct {
	db           *pgxpool.Pool
	commitCache  *lru.Cache
	cacheClient  cache.Cache
	windowLength int
}

func commitKey(commitId schema.CommitID) string {
	return fmt.Sprintf("commit_%s", commitId)
}

// NewCommitsProvider returns a new instance of CommitsProvider.
func NewCommitsProvider(db *pgxpool.Pool, cacheClient cache.Cache, windowLength int) *CommitsProvider {
	cc, err := lru.New(commitCacheSize)
	if err != nil {
		panic(err) // should only happen if commitCacheSize is negative.
	}
	return &CommitsProvider{
		cacheClient:  cacheClient,
		db:           db,
		commitCache:  cc,
		windowLength: windowLength,
	}
}

// GetCommits returns the front-end friendly version of the commits within the searched window.
func (s *CommitsProvider) GetCommits(ctx context.Context) ([]frontend.Commit, error) {
	ctx, span := trace.StartSpan(ctx, "getCommits")
	defer span.End()
	rv := make([]frontend.Commit, common.GetActualWindowLength(ctx))
	commitIDs := common.GetCommitToIdxMap(ctx)
	for commitID, idx := range commitIDs {
		var commit frontend.Commit
		commit, err := s.getFromCache(ctx, commitID)
		if err == nil {
			// This means commit has been retrieved from remote cache.
			// Continue with the loop.
			rv[idx] = commit
			continue
		}

		// If the execution has reached here, either a remote cache is not configured
		// or invalid data was retrieved from cache. Fall back to the older method of
		// getting commits.
		if c, ok := s.commitCache.Get(commitID); ok {
			commit = c.(frontend.Commit)
		} else {
			if isStandardGitCommitID(commitID) {
				const statement = `SELECT git_hash, commit_time, author_email, subject
FROM GitCommits WHERE commit_id = $1`
				row := s.db.QueryRow(ctx, statement, commitID)
				var dbRow schema.GitCommitRow
				if err := row.Scan(&dbRow.GitHash, &dbRow.CommitTime, &dbRow.AuthorEmail, &dbRow.Subject); err != nil {
					return nil, skerr.Wrap(err)
				}
				commit = frontend.Commit{
					CommitTime: dbRow.CommitTime.UTC().Unix(),
					ID:         string(commitID),
					Hash:       dbRow.GitHash,
					Author:     dbRow.AuthorEmail,
					Subject:    dbRow.Subject,
				}
				s.commitCache.Add(commitID, commit)
			} else {
				commit = frontend.Commit{
					ID: string(commitID),
				}
				s.commitCache.Add(commitID, commit)
			}
		}
		rv[idx] = commit
	}
	return rv, nil
}

// GetCommitsInWindow implements the API interface
func (s *CommitsProvider) GetCommitsInWindow(ctx context.Context) ([]frontend.Commit, error) {
	const statement = `WITH
RecentCommits AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
)
SELECT git_hash, GitCommits.commit_id, commit_time, author_email, subject FROM GitCommits
JOIN RecentCommits ON GitCommits.commit_id = RecentCommits.commit_id
ORDER BY commit_id ASC`
	rows, err := s.db.Query(ctx, statement, s.windowLength)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []frontend.Commit
	for rows.Next() {
		var ts time.Time
		var commit frontend.Commit
		if err := rows.Scan(&commit.Hash, &commit.ID, &ts, &commit.Author, &commit.Subject); err != nil {
			return nil, skerr.Wrap(err)
		}
		commit.CommitTime = ts.UTC().Unix()
		rv = append(rv, commit)
	}
	return rv, nil
}

// isStandardGitCommitID detects our standard commit ids for git repos (monotonically increasing
// integers). It returns false if that is not being used (e.g. for instances that don't use that
// as their ID)
func isStandardGitCommitID(id schema.CommitID) bool {
	_, err := strconv.ParseInt(string(id), 10, 64)
	return err == nil
}

// PopulateCommitCache retrieves commits in the current window and adds them
// to the configured cache for the instance.
func (s *CommitsProvider) PopulateCommitCache(ctx context.Context) error {
	if s.cacheClient != nil {
		commits, err := s.GetCommitsInWindow(ctx)
		if err != nil {
			sklog.Errorf("Error getting commits in the current window: %v", err)
			return skerr.Wrap(err)
		}

		sklog.Infof("Populating %d commits to the cache", len(commits))
		for _, commit := range commits {
			json, err := common.ToJSON(commit)
			if err != nil {
				sklog.Errorf("Error converting commit to json string: %v", err)
				return skerr.Wrap(err)
			}

			err = s.cacheClient.SetValue(ctx, commitKey(schema.CommitID(commit.ID)), json)
			if err != nil {
				sklog.Errorf("Error setting commit data in the cache: %v", err)
				return skerr.Wrap(err)
			}
		}
	}

	return nil
}

// getFromCache returns the commit for the given commitID from the cache.
func (s *CommitsProvider) getFromCache(ctx context.Context, commitID schema.CommitID) (frontend.Commit, error) {
	if s.cacheClient != nil {
		value, err := s.cacheClient.GetValue(ctx, commitKey(commitID))
		commit := frontend.Commit{}
		if err != nil {
			sklog.Errorf("Error retrieving commit information from cache: %v", err)
			return commit, skerr.Wrap(err)
		}

		if value == "" {
			sklog.Errorf("Empty commit data returned from cache.")
			return commit, skerr.Fmt("Empty commit data returned from cache")
		}
		err = json.Unmarshal([]byte(value), &commit)
		if err != nil {
			sklog.Errorf("Error converting json str %s to commit object: %v", value, err)
			return commit, skerr.Wrap(err)
		}

		return commit, nil
	}

	sklog.Infof("Cache is not configured for this instance.")
	return frontend.Commit{}, skerr.Fmt("Cache is not configured for this instance.")
}
