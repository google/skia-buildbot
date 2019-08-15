package bt_gitstore

import (
	"sort"
	"strings"
	"sync/atomic"

	"cloud.google.com/go/bigtable"
	multierror "github.com/hashicorp/go-multierror"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
)

// shardedResults collects the results of call to iterShardedRange.
type shardedResults interface {
	// Add adds the result of a shard.
	Add(shard uint32, row bigtable.Row) error

	// Finish indicates that the shard is done processing its results.
	Finish(shard uint32)
}

// srIndexCommits implements the shardedResults interface for collecting results that are vcsinfo.IndexCommits
type srIndexCommits struct {
	errs    []*multierror.Error
	results [][]*vcsinfo.IndexCommit
	retSize int64
}

// create a new instance to collect IndexCommits.
func newSRIndexCommits(shards uint32) *srIndexCommits {
	return &srIndexCommits{
		results: make([][]*vcsinfo.IndexCommit, shards),
		errs:    make([]*multierror.Error, shards),
	}
}

// Add implements the shardedResults interface.
func (s *srIndexCommits) Add(shard uint32, row bigtable.Row) error {
	idx := parseIndex(keyFromRowName(row.Key()))
	if idx < 0 {
		return skerr.Fmt("Unable to parse index key %q. Invalid index", row.Key())
	}
	prefix := cfCommit + ":"
	for _, col := range row[cfCommit] {
		if strings.TrimPrefix(col.Column, prefix) == colHash {
			s.results[shard] = append(s.results[shard], &vcsinfo.IndexCommit{
				Index:     idx,
				Hash:      string(col.Value),
				Timestamp: col.Timestamp.Time().UTC(),
			})
		}
	}
	return nil
}

// Finish implements the shardedResults interface.
func (s *srIndexCommits) Finish(shard uint32) {
	atomic.AddInt64(&s.retSize, int64(len(s.results)))
}

// Sorted returns the resulting IndexCommits by Index->TimeStamp->Hash.
// Using the hash ensures results with identical timestamps are sorted stably.
func (s *srIndexCommits) Sorted() []*vcsinfo.IndexCommit {
	// Concatenate the shard results into a single output and sort it.
	ret := make([]*vcsinfo.IndexCommit, 0, s.retSize)
	for _, sr := range s.results {
		ret = append(ret, sr...)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Index < ret[j].Index ||
			((ret[i].Index == ret[j].Index) && ret[i].Timestamp.Before(ret[j].Timestamp)) ||
			((ret[i].Index == ret[j].Index) && ret[i].Timestamp.Equal(ret[j].Timestamp) &&
				(ret[i].Hash < ret[j].Hash))
	})
	return ret
}

// srTimestampCommits is an adaptation of srIndexCommits that extracts a different
// column family from a timestamp-based index. Otherwise it is identical.
type srTimestampCommits struct {
	*srIndexCommits
}

// newSRTimestampCommits creates a new instance to receive timestamp based IndexCommits.
func newSRTimestampCommits(shards uint32) *srTimestampCommits {
	return &srTimestampCommits{
		srIndexCommits: newSRIndexCommits(shards),
	}
}

// Add implements the shardedResults interface and overrides the Add function in srIndexCommits.
func (s *srTimestampCommits) Add(shard uint32, row bigtable.Row) error {
	prefix := cfTsCommit + ":"
	for _, col := range row[cfTsCommit] {
		hash := strings.TrimPrefix(col.Column, prefix)
		timeStamp := col.Timestamp

		// Parse the index
		idxStr := string(col.Value)
		idx := parseIndex(idxStr)
		if idx < 0 {
			return skerr.Fmt("Unable to parse index key %q. Invalid index", idxStr)
		}

		s.results[shard] = append(s.results[shard], &vcsinfo.IndexCommit{
			Hash:      hash,
			Timestamp: timeStamp.Time().UTC(),
			Index:     idx,
		})
	}
	return nil
}
