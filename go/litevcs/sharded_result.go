package litevcs

import (
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigtable"
	multierror "github.com/hashicorp/go-multierror"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

type shardedResults interface {
	Add(shard uint32, row bigtable.Row) error
	Finish(shard uint32)
}

type srIndexCommits struct {
	errs    []*multierror.Error
	results [][]*vcsinfo.IndexCommit
	retSize int64
}

func newSRIndexCommits(shards uint32) *srIndexCommits {
	return &srIndexCommits{
		results: make([][]*vcsinfo.IndexCommit, shards),
		errs:    make([]*multierror.Error, shards),
	}
}

func (s *srIndexCommits) Add(shard uint32, row bigtable.Row) error {
	sklog.Infof("AAAA: %s", row.Key())
	idx := parseIndex(keyFromRowName(row.Key()))
	if idx < 0 {
		return skerr.Fmt("Unable to parse index key %q. Invalid index", row.Key())
	}

	var hash string
	var timeStamp bigtable.Timestamp
	prefix := cfCommit + ":"
	for _, col := range row[cfCommit] {
		if strings.TrimPrefix(col.Column, prefix) == colHash {
			hash = string(col.Value)
			timeStamp = col.Timestamp
		}
	}

	s.results[shard] = append(s.results[shard], &vcsinfo.IndexCommit{
		Index:     idx,
		Hash:      hash,
		Timestamp: timeStamp.Time().UTC(),
	})
	return nil
}

func (s *srIndexCommits) Finish(shard uint32) {
	atomic.AddInt64(&s.retSize, int64(len(s.results)))
}

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

type srTimestampCommits struct {
	*srIndexCommits
}

func newSRTimestampCommits(shards uint32) *srTimestampCommits {
	return &srTimestampCommits{
		srIndexCommits: newSRIndexCommits(shards),
	}
}

func (s *srTimestampCommits) Add(shard uint32, row bigtable.Row) error {
	sklog.Infof("RRRRR: %s", row.Key())
	prefix := cfTsCommit + ":"
	for _, col := range row[cfTsCommit] {
		sklog.Infof("ROW: %s", col.Column)
		hash := strings.TrimPrefix(col.Column, prefix)
		timeStamp := col.Timestamp

		s.results[shard] = append(s.results[shard], &vcsinfo.IndexCommit{
			Hash:      hash,
			Timestamp: timeStamp.Time().UTC(),
		})
	}
	return nil
}

type rawNodesResult struct {
	errs       []*multierror.Error
	results    [][][]string
	timeStamps [][]time.Time
	retSize    int64
}

func newRawNodesResult(shards uint32) *rawNodesResult {
	return &rawNodesResult{
		results:    make([][][]string, shards),
		timeStamps: make([][]time.Time, shards),
		errs:       make([]*multierror.Error, shards),
	}
}

var rawNodeColPrefix = cfCommit + ":"

func (r *rawNodesResult) Add(shard uint32, row bigtable.Row) error {
	var commitHash string
	var parents []string
	var timeStamp bigtable.Timestamp
	for _, col := range row[cfCommit] {
		switch strings.TrimPrefix(col.Column, rawNodeColPrefix) {
		case colHash:
			commitHash = string(col.Value)
			timeStamp = col.Timestamp
		case colParents:
			if len(col.Value) > 0 {
				parents = strings.Split(string(col.Value), ":")
			}
		}
	}
	hp := make([]string, 0, 1+len(parents))
	hp = append(hp, commitHash)
	hp = append(hp, parents...)
	r.results[shard] = append(r.results[shard], hp)
	r.timeStamps[shard] = append(r.timeStamps[shard], timeStamp.Time())
	return nil
}

func (r *rawNodesResult) Finish(shard uint32) {
	atomic.AddInt64(&r.retSize, int64(len(r.results)))
}

func (r *rawNodesResult) Merge() ([][]string, []time.Time) {
	ret := make([][]string, 0, r.retSize)
	timeStamps := make([]time.Time, 0, r.retSize)
	for idx, shardResults := range r.results {
		ret = append(ret, shardResults...)
		timeStamps = append(timeStamps, r.timeStamps[idx]...)
	}
	return ret, timeStamps
}
