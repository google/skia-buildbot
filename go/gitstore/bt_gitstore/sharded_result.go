package bt_gitstore

import (
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

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
	errs       []*multierror.Error
	timestamps []map[*vcsinfo.IndexCommit]time.Time
	results    [][]*vcsinfo.IndexCommit
	retSize    int64
}

// create a new instance to collect IndexCommits.
func newSRIndexCommits(shards uint32) *srIndexCommits {
	timestamps := make([]map[*vcsinfo.IndexCommit]time.Time, shards)
	for i := uint32(0); i < shards; i++ {
		timestamps[i] = map[*vcsinfo.IndexCommit]time.Time{}
	}
	return &srIndexCommits{
		results:    make([][]*vcsinfo.IndexCommit, shards),
		timestamps: timestamps,
		errs:       make([]*multierror.Error, shards),
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
		if strings.TrimPrefix(col.Column, prefix) == colHashTs {
			split := strings.Split(string(col.Value), "#")
			if len(split) != 2 {
				return skerr.Fmt("Unable to parse hash#timestamp from %q", string(col.Value))
			}
			ts, err := strconv.Atoi(split[1])
			if err != nil {
				return skerr.Wrapf(err, "Unable to parse hash#timestamp from %q", string(col.Value))
			}
			hash := make([]byte, len(split[0]))
			copy(hash, []byte(split[0]))
			ic := &vcsinfo.IndexCommit{
				Index:     idx,
				Hash:      string(hash),
				Timestamp: time.Unix(int64(ts), 0).UTC(), // Git has a timestamp resolution of 1s.
			}
			s.results[shard] = append(s.results[shard], ic)
			s.timestamps[shard][ic] = col.Timestamp.Time().UTC()
		}
	}
	return nil
}

// Finish implements the shardedResults interface.
func (s *srIndexCommits) Finish(shard uint32) {
	atomic.AddInt64(&s.retSize, int64(len(s.results)))
}

// Sorted returns the resulting IndexCommits by Index->TimeStamp->Hash, and the
// server-side update timestamps associated with each IndexCommit.
// Using the hash ensures results with identical timestamps are sorted stably.
func (s *srIndexCommits) Sorted() ([]*vcsinfo.IndexCommit, map[*vcsinfo.IndexCommit]time.Time) {
	// Concatenate the shard results into a single output and sort it.
	ics := make([]*vcsinfo.IndexCommit, 0, s.retSize)
	timestamps := make(map[*vcsinfo.IndexCommit]time.Time, len(ics))
	for shard, sr := range s.results {
		ics = append(ics, sr...)
		for ic, t := range s.timestamps[shard] {
			timestamps[ic] = t
		}
	}
	sort.Sort(vcsinfo.IndexCommitSlice(ics))
	return ics, timestamps
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

		// Parse the timestamp from the row key.
		split := strings.Split(row.Key(), ":")
		ts, err := strconv.Atoi(split[len(split)-1])
		if err != nil {
			return skerr.Wrapf(err, "Unable to parse timestamp from row key %q", row.Key())
		}

		// Parse the index.
		idxStr := string(col.Value)
		idx := parseIndex(idxStr)
		if idx < 0 {
			return skerr.Fmt("Unable to parse index key %q. Invalid index", idxStr)
		}

		ic := &vcsinfo.IndexCommit{
			Hash:      hash,
			Timestamp: time.Unix(int64(ts), 0).UTC(), // Git has a timestamp resolution of 1s.
			Index:     idx,
		}
		s.results[shard] = append(s.results[shard], ic)
		s.timestamps[shard][ic] = col.Timestamp.Time().UTC()
	}
	return nil
}
