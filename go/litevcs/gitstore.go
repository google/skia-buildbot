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
	multierror "github.com/hashicorp/go-multierror"

	"go.skia.org/infra/go/bt"
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
	typIndex     = "i"
	typTimeStamp = "z"
)

var (
	minTime = time.Time{}
	maxTime = time.Unix(int64(^uint64(0)>>1), 0)
	maxInt  = int(^uint(0) >> 1)

	ColumnFamilies = []string{cfCommit}
)

func InitBT(conf *BTConfig) error {
	return bt.InitBigtable(conf.ProjectID, conf.InstanceID, conf.TableID, []string{cfCommit})
}

type GitStore interface {
	Put(ctx context.Context, commits []*vcsinfo.LongCommit, commitIndices []int) error
	Get(ctx context.Context, hash []string, indexOnly bool) ([]*vcsinfo.LongCommit, []int, error)
	RangeN(ctx context.Context, startIndex, endIndex int) ([]*vcsinfo.IndexCommit, error)
	RangeByTime(ctx context.Context, start, end time.Time) ([]*vcsinfo.IndexCommit, error)
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

func (b *btGitStore) Put(ctx context.Context, commits []*vcsinfo.LongCommit, commitIndices []int) error {
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

		tsRowNames = append(tsRowNames, b.rowName(typTimeStamp, searchableTimestamp(commit.Timestamp)))
		tsMutations = append(tsMutations, b.simpleMutation(commit.Timestamp, [][2]string{
			{colHash, commit.Hash},
			{colIndex, sIndex},
		}...))

		idxRowNames = append(idxRowNames, b.rowName(typIndex, sIndex))
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

type shardedResults interface {
	Add(shard int32, row bigtable.Row) error
	Finish(shard int32)
}

type srIndexCommits struct {
	errs    []*multierror.Error
	results [][]*vcsinfo.IndexCommit
	retSize int64
}

func newSRIndexCommits(shards int32) *srIndexCommits {
	return &srIndexCommits{
		results: make([][]*vcsinfo.IndexCommit, shards),
		errs:    make([]*multierror.Error, shards),
	}
}

func (s *srIndexCommits) Add(shard int32, row bigtable.Row) error {
	idx := parseIndex(extractKey(row.Key()))
	if idx < 0 {
		return skerr.Fmt("Unable to parse index key %q. Invalid index", row.Key())
	}

	var hash string
	var timeStamp bigtable.Timestamp
	for _, col := range row[cfCommit] {
		if col.Column == colHash {
			hash = string(col.Value)
			timeStamp = col.Timestamp
		}
	}

	s.results[shard] = append(s.results[shard], &vcsinfo.IndexCommit{
		Index:     idx,
		Hash:      hash,
		Timestamp: timeStamp.Time(),
	})
	return nil
}

func (s *srIndexCommits) Finish(shard int32) {
	atomic.AddInt64(&s.retSize, int64(len(s.results)))
}

func (s *srIndexCommits) Sorted() []*vcsinfo.IndexCommit {
	// Concatenate the shard results into a single output and sort it.
	ret := make([]*vcsinfo.IndexCommit, 0, s.retSize)
	for _, sr := range s.results {
		ret = append(ret, sr...)
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].Index < ret[j].Index })
	return ret
}

func (b *btGitStore) RangeByTime(ctx context.Context, start, end time.Time) ([]*vcsinfo.IndexCommit, error) {
	startTS := searchableTimestamp(start)
	endTS := searchableTimestamp(end)

	result := newSRIndexCommits(b.shards)
	err := b.iterShardedRange(ctx, startTS, endTS, typTimeStamp, colHash, result)
	if err != nil {
		return nil, err
	}

	return result.Sorted(), nil
}

func (b *btGitStore) RangeN(ctx context.Context, startIndex, endIndex int) ([]*vcsinfo.IndexCommit, error) {
	startTS := searchableIndex(startIndex)
	endTS := searchableIndex(endIndex)

	result := newSRIndexCommits(b.shards)
	err := b.iterShardedRange(ctx, startTS, endTS, typTimeStamp, colHash, result)
	if err != nil {
		return nil, err
	}

	return result.Sorted(), nil
}

func (b *btGitStore) iterShardedRange(ctx context.Context, startKey, endKey, rowType string, colFilter string, result shardedResults) error {
	var egroup errgroup.Group

	// Set up the filter for the query
	filters := []bigtable.Filter{bigtable.FamilyFilter(cfCommit)}
	if colFilter != "" {
		filters = append(filters, bigtable.ColumnFilter(colFilter))
	}
	rowFilters := bigtable.RowFilter(bigtable.ChainFilters(filters...))

	for shard := int32(0); shard < b.shards; shard++ {
		func(shard int32) {
			egroup.Go(func() error {
				defer result.Finish(shard)

				rStart := shardedRowName(shard, rowType, startKey)
				rEnd := shardedRowName(shard, rowType, endKey)
				rr := bigtable.NewRange(rStart, rEnd)

				var addErr error
				err := b.table.ReadRows(ctx, rr, func(row bigtable.Row) bool {
					addErr = result.Add(shard, row)
					return addErr == nil
				}, rowFilters)
				if err != nil {
					return err
				}
				return addErr
			})
		}(shard)
	}

	if err := egroup.Wait(); err != nil {
		return err
	}
	return nil
}

func (b *btGitStore) simpleMutation(timeStamp time.Time, colValPairs ...[2]string) *bigtable.Mutation {
	ts := bigtable.Time(timeStamp)
	ret := bigtable.NewMutation()
	for _, pair := range colValPairs {
		ret.Set(cfCommit, pair[0], ts, []byte(pair[1]))
	}
	return ret
}

func (b *btGitStore) Get(ctx context.Context, hashes []string, indexOnly bool) ([]*vcsinfo.LongCommit, []int, error) {
	rowNames := make(bigtable.RowList, len(hashes))
	for idx, h := range hashes {
		rowNames[idx] = b.rowName("", h)
	}

	ret := make([]*vcsinfo.LongCommit, 0, len(hashes))
	retIdx := make([]int, 0, len(hashes))

	err := b.table.ReadRows(ctx, rowNames, func(row bigtable.Row) bool {
		commit := &vcsinfo.LongCommit{}
		commit.Hash = extractKey(row.Key())

		for _, col := range row[cfCommit] {
			switch col.Column {
			case colAuthor:
				commit.Author = string(col.Value)
				commit.Timestamp = col.Timestamp.Time()
			case colSubject:
				commit.Subject = string(col.Value)
			case colParents:
				strings.Split(string(col.Value), ":")
			case colBody:
				commit.Body = string(col.Value)
			case colIndex:
				retIdx = append(retIdx, parseIndex(string(col.Value)))
			}
		}
		ret = append(ret, commit)
		return true
	})

	if err != nil {
		return nil, nil, err
	}
	return ret, retIdx, nil
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
