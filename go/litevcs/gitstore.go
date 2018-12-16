package litevcs

import (
	"context"
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/sync/errgroup"
)

const (
	cfCommit = "C"

	// index commit
	colHash      = "h"
	colIndex     = "i"
	colTimestamp = "t"

	// shortcommit
	colAuthor   = "a"
	colSubject  = "s"
	colParents  = "p"
	colBody     = "b"
	colBranches = "br"

	// Define the 'row'types.
	tIndex     = "i"
	tTimeStamp = "z"
)

var (
	minTime = time.Time{}
	maxTime = time.Unix(int64(^uint64(0)>>1), 0)
	maxInt  = int(^uint(0) >> 1)

	ColumnFamilies = []string{cfCommit}
)

type GitStore interface {
	Put(commits []*vcsinfo.LongCommit, commitIndices []int) error
	Get(hash []string, indexOnly bool) ([]*vcsinfo.LongCommit, []int, error)
	RangeN(startIndex, endIndex int) ([]*vcsinfo.IndexCommit, error)
	RangeByTime(start, end time.Time) ([]*vcsinfo.IndexCommit, error)
}

type BTConfig struct {
	ProjectID  string
	InstanceID string
	TableID    string
	Shards     int32
}

type btGitStore struct {
	table  *bigtable.Table
	shards int32
}

func NewBTGitStore(config *BTConfig) (GitStore, error) {
	// Create the client.
	client, err := bigtable.NewClient(context.TODO(), config.ProjectID, config.InstanceID)
	if err != nil {
		return nil, skerr.Fmt("Error creating bigtable client: %s", err)
	}

	ret := &btGitStore{
		table:  client.Open(config.TableID),
		shards: config.Shards,
	}
	return ret, nil
}

func (b *btGitStore) Put(commits []*vcsinfo.LongCommit, commitIndices []int) error {
	// Assemble the mutations.
	nMutations := len(commits)
	rowNames := make([]string, 0, nMutations)
	mutations := make([]*bigtable.Mutation, 0, nMutations)
	tsRowNames := make([]string, 0, nMutations)
	tsMutations := make([]*bigtable.Mutation, 0, nMutations)
	idxRowNames := make([]string, 0, nMutations)
	idxMutations := make([]*bigtable.Mutation, 0, nMutations)
	for idx, commit := range commits {
		sIndex := searchableIndex(commitIndices[idx])

		rowNames = append(rowNames, b.rowName("", commit.Hash))
		mutations = append(mutations, b.getCommitMutation(commit, sIndex))

		tsRowNames = append(tsRowNames, b.rowName(tTimeStamp, searchableTimestamp(commit.Timestamp)))
		tsMutations = append(tsMutations, b.simpleMutation(commit.Timestamp, [][2]string{
			{colHash, commit.Hash},
			{colIndex, sIndex},
		}...))

		idxRowNames = append(idxRowNames, b.rowName(tIndex, sIndex))
		idxMutations = append(idxMutations, b.simpleMutation(commit.Timestamp, [2]string{colHash, commit.Hash}))
	}

	sklog.Infof("TABLE IS NULL: %v", b.table == nil)

	errs, err := b.table.ApplyBulk(context.TODO(), rowNames, mutations)
	if err != nil {
		return skerr.Fmt("Error writing commits: %s", err)
	}
	if errs != nil {
		return skerr.Fmt("Error writing some commits: %s", errs)
	}

	errs, err = b.table.ApplyBulk(context.TODO(), tsRowNames, tsMutations)
	if err != nil {
		return skerr.Fmt("Error writing timestamps: %s", err)
	}
	if errs != nil {
		return skerr.Fmt("Error writing some timestamps: %s", errs)
	}

	errs, err = b.table.ApplyBulk(context.TODO(), idxRowNames, idxMutations)
	if err != nil {
		return skerr.Fmt("Error writing indices: %s", err)
	}
	if errs != nil {
		return skerr.Fmt("Error writing some indices: %s", errs)
	}
	return nil
}

func (b *btGitStore) RangeByTime(start, end time.Time) ([]*vcsinfo.IndexCommit, error) {
	startTS := searchableTimestamp(start)
	endTS := searchableTimestamp(end)

	b.filterShardedRange(startTS, endTS, typTimeStamp)

}



func (b *btGitStore) filterShardedRange(startKey, endKey string) {
	startTS := searchableTimestamp(start)
	endTS := searchableTimestamp(end)
	ctx := context.TODO()

	results := make([][]*vcsinfo.IndexCommit, b.shards)
	var egroup errgroup.Group
	retSize := int64(0)
	for shard := int32(0); shard < b.shards; shard++ {
		func(shard int32) {
			egroup.Go(func() error {
				rStart := shardedRowName(shard, tTimeStamp, startTS)
				rEnd := shardedRowName(shard, tTimeStamp, endTS)
				// sklog.Infof("Range: %q    %q", rStart, rEnd)
				rr := bigtable.NewRange(rStart, rEnd)
				shardResult := []*vcsinfo.IndexCommit{}
				rowFilters := bigtable.RowFilter(bigtable.ChainFilters(
					bigtable.FamilyFilter(cfCommit),
					bigtable.ColumnFilter(colHash)))

				err := b.table.ReadRows(ctx, rr, func(row bigtable.Row) bool {
					idx := parseIndex(extractKey(row.Key()))
					var hash string
					var timeStamp bigtable.Timestamp
					for _, col := range row[cfCommit] {
						// sklog.Infof("COL: %s", spew.Sdump(col))
						if col.Column == colHash {
							hash = string(col.Value)
							timeStamp = col.Timestamp
						}
					}

					shardResult = append(shardResult, &vcsinfo.IndexCommit{
						Index:     idx,
						Hash:      hash,
						Timestamp: timeStamp.Time(),
					})

					return true
				}, rowFilters)
				if err != nil {
					return err
				}

				results[shard] = shardResult
				atomic.AddInt64(&retSize, int64(len(shardResult)))
				return nil
			})
		}(shard)
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	// Concatenate the shard results into a single output and sort it.
	ret := make([]*vcsinfo.IndexCommit, 0, retSize)
	for _, sr := range results {
		ret = append(ret, sr...)
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].Index < ret[j].Index })

	if len(ret) > 0 && ret[0].Index < 0 {
		return nil, skerr.Fmt("Unable to parse index key. Internal programming error")
	}

	return ret, nil
}

func filterShardedRange(startKey, endKey string) {
	startTS := searchableTimestamp(start)
	endTS := searchableTimestamp(end)
	ctx := context.TODO()

	results := make([][]*vcsinfo.IndexCommit, b.shards)
	var egroup errgroup.Group
	retSize := int64(0)
	for shard := int32(0); shard < b.shards; shard++ {
		func(shard int32) {
			egroup.Go(func() error {
				rStart := shardedRowName(shard, tTimeStamp, startTS)
				rEnd := shardedRowName(shard, tTimeStamp, endTS)
				// sklog.Infof("Range: %q    %q", rStart, rEnd)
				rr := bigtable.NewRange(rStart, rEnd)
				shardResult := []*vcsinfo.IndexCommit{}
				rowFilters := bigtable.RowFilter(bigtable.ChainFilters(
					bigtable.FamilyFilter(cfCommit),
					bigtable.ColumnFilter(colHash)))

				err := b.table.ReadRows(ctx, rr, func(row bigtable.Row) bool {
					idx := parseIndex(extractKey(row.Key()))
					var hash string
					var timeStamp bigtable.Timestamp
					for _, col := range row[cfCommit] {
						// sklog.Infof("COL: %s", spew.Sdump(col))
						if col.Column == colHash {
							hash = string(col.Value)
							timeStamp = col.Timestamp
						}
					}

					shardResult = append(shardResult, &vcsinfo.IndexCommit{
						Index:     idx,
						Hash:      hash,
						Timestamp: timeStamp.Time(),
					})

					return true
				}, rowFilters)
				if err != nil {
					return err
				}

				results[shard] = shardResult
				atomic.AddInt64(&retSize, int64(len(shardResult)))
				return nil
			})
		}(shard)
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	// Concatenate the shard results into a single output and sort it.
	ret := make([]*vcsinfo.IndexCommit, 0, retSize)
	for _, sr := range results {
		ret = append(ret, sr...)
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].Index < ret[j].Index })

	if len(ret) > 0 && ret[0].Index < 0 {
		return nil, skerr.Fmt("Unable to parse index key. Internal programming error")
	}

	return ret, nil
}


func (b *btGitStore) RangeN(startIndex, endIndex int) ([]*vcsinfo.IndexCommit, error) {
	return nil, nil
}

func (b *btGitStore) simpleMutation(timeStamp time.Time, colValPairs ...[2]string) *bigtable.Mutation {
	ts := bigtable.Time(timeStamp)
	ret := bigtable.NewMutation()
	for _, pair := range colValPairs {
		ret.Set(cfCommit, pair[0], ts, []byte(pair[1]))
	}
	return ret
}

func (b *btGitStore) Get(hashes []string, indexOnly bool) ([]*vcsinfo.LongCommit, []int, error) {
	return nil, nil, nil
}

func (b *btGitStore) CommitRange(start, end time.Time) ([]string, error) {
	return nil, nil
}

func (b *btGitStore) getCommitMutation(commit *vcsinfo.LongCommit, commitIndex string) *bigtable.Mutation {
	ts := bigtable.Time(commit.Timestamp)
	ret := bigtable.NewMutation()
	ret.Set(cfCommit, colHash, ts, []byte(commit.Hash))
	ret.Set(cfCommit, colAuthor, ts, []byte(commit.Author))
	ret.Set(cfCommit, colSubject, ts, []byte(commit.Subject))
	ret.Set(cfCommit, colParents, ts, []byte(strings.Join(commit.Parents, ":")))
	ret.Set(cfCommit, colBody, ts, []byte(commit.Body))
	ret.Set(cfCommit, colIndex, ts, []byte(commitIndex))
	return ret
}

func (b *btGitStore) rowName(rowType string, key string) string {
	return fmt.Sprintf("%d:%s%s", crc32.ChecksumIEEE([]byte(key))%uint32(b.shards), rowType, key)
}

func shardedRowName(shard int32, rowType, key string) string {
	return fmt.Sprintf("%d:%s%s", shard, rowType, key)
}

func extractKey(rowName string) string {
	parts := strings.SplitN(rowName, ":", 2)
	return parts[1][1:]
}

func searchableTimestamp(ts time.Time) string {
	return fmt.Sprintf("%012d", util.MinInt64(999999999999, ts.Unix()))
}

func searchableIndex(index int) string {
	return fmt.Sprintf("%08d", util.MinInt(999999999, index))
}

func parseIndex(indexStr string) int {
	ret, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		return -1
	}
	return int(ret)
}
