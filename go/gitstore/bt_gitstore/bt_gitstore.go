package bt_gitstore

// The bt_gitstore package implements a way to store Git metadata in BigTable for faster retrieval
// than requiring a local checkout.

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/sync/errgroup"
)

// BigTableGitStore implements the GitStore interface based on BigTable.
type BigTableGitStore struct {
	RepoID  int64
	RepoURL string
	shards  uint32
	table   *bigtable.Table
}

// New returns an instance of GitStore that uses BigTable as its backend storage.
// The given repoURL serves to identify the repository. Internally it is stored normalized
// via a call to git.NormalizeURL.
func New(ctx context.Context, config *BTConfig, repoURL string) (*BigTableGitStore, error) {
	// Create the client.
	client, err := bigtable.NewClient(ctx, config.ProjectID, config.InstanceID)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating bigtable client (project: %s; instance: %s)", config.ProjectID, config.InstanceID)
	}

	repoURL, err = git.NormalizeURL(repoURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "normalizing URL %q", repoURL)
	}

	shards := config.Shards
	if shards <= 0 {
		shards = DefaultShards
	}

	ret := &BigTableGitStore{
		table:   client.Open(config.TableID),
		shards:  uint32(shards),
		RepoURL: repoURL,
	}

	repoInfo, err := ret.loadRepoInfo(ctx, true)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting initial repo info for %s (project %s; instance: %s)", repoURL, config.ProjectID, config.InstanceID)
	}
	ret.RepoID = repoInfo.ID
	return ret, nil
}

const (
	// Column families.
	cfBranches = "B"
	cfCommit   = "C"
	cfTsCommit = "T"
	cfMeta     = "M"

	// meta data columns.
	colMetaID        = "metaID"
	colMetaIDCounter = "metaIDCounter"

	// Keys of meta data rows.
	metaVarRepo      = "repo"
	metaVarIDCounter = "idcounter"

	// index commit
	colHash      = "h"
	colTimestamp = "t"

	// long commit
	colAuthor   = "a"
	colSubject  = "s"
	colParents  = "p"
	colBody     = "b"
	colBranches = "br"
	colIndex    = "i"

	// Define the row types.
	typIndex     = "i"
	typTimeStamp = "z"
	typCommit    = "k"
	typMeta      = "!"

	// getBatchSize is the batchsize for the Get operation. Each call to bigtable is made with maximally
	// this number of git hashes. This is a conservative number to stay within the 1M request
	// size limit. Since requests are sharded this will not limit throughput in practice.
	getBatchSize = 5000

	// writeBatchSize is the number of mutations to write at once. This could be fine tuned for different
	// row types.
	writeBatchSize = 1000
)

var (
	// Default number of shards used, if not shards provided in BTConfig.
	DefaultShards = 32
)

// Put implements the GitStore interface.
func (b *BigTableGitStore) Put(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	if len(commits) == 0 {
		return nil
	}

	// Organize the commits by branch.
	indexCommitsByBranch := map[string][]*vcsinfo.IndexCommit{}
	for _, c := range commits {
		// Validation.
		if c.Index == 0 && len(c.Parents) != 0 {
			return skerr.Fmt("Commit %s has index zero but has at least one parent. This cannot be correct.", c.Hash)
		}
		if len(c.Branches) == 0 {
			// TODO(borenet): Is there any way to check for this?
			sklog.Warningf("Commit %s has no branch information; this is valid if it is not on the first-parent ancestry chain of any branch.", c.Hash)
		}
		// Create the IndexCommit(s).
		ic := &vcsinfo.IndexCommit{
			Hash:      c.Hash,
			Index:     c.Index,
			Timestamp: c.Timestamp,
		}
		indexCommitsByBranch[gitstore.ALL_BRANCHES] = append(indexCommitsByBranch[gitstore.ALL_BRANCHES], ic)
		for branch := range c.Branches {
			if branch != gitstore.ALL_BRANCHES {
				indexCommitsByBranch[branch] = append(indexCommitsByBranch[branch], ic)
			}
		}
	}

	// Write the LongCommits and IndexCommits to BT.
	// TODO(borenet): Could we first obtain the list of mutations and then
	// run a big set of batched updates?
	if err := b.writeLongCommits(ctx, commits); err != nil {
		return skerr.Wrapf(err, "Error writing long commits.")
	}
	var egroup errgroup.Group
	for branch, indexCommits := range indexCommitsByBranch {
		// https://golang.org/doc/faq#closures_and_goroutines
		branch := branch
		indexCommits := indexCommits
		egroup.Go(func() error {
			if err := b.writeTimestampIndex(ctx, indexCommits, branch); err != nil {
				return err
			}
			return b.writeIndexCommits(ctx, indexCommits, branch)
		})
	}
	if err := egroup.Wait(); err != nil {
		return skerr.Wrapf(err, "Error writing index commits.")
	}
	allCommits := indexCommitsByBranch[gitstore.ALL_BRANCHES]
	sort.Sort(vcsinfo.IndexCommitSlice(allCommits))
	return b.putBranchPointer(ctx, getRepoInfoRowName(b.RepoURL), gitstore.ALL_BRANCHES, allCommits[len(allCommits)-1])
}

// Get implements the GitStore interface.
func (b *BigTableGitStore) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error) {
	// hashOrder tracks the original index(es) of each hash in the passed-in
	// slice. It is used to ensure that we return the LongCommits in the
	// desired order, despite our receiving them from BT in arbitrary order.
	hashOrder := make(map[string][]int, len(hashes))
	for idx, h := range hashes {
		hashOrder[h] = append(hashOrder[h], idx)
	}
	rowNames := make(bigtable.RowList, 0, len(hashOrder))
	for h := range hashOrder {
		rowNames = append(rowNames, b.rowName("", typCommit, h))
	}

	var egroup errgroup.Group
	tempRet := make([]*vcsinfo.LongCommit, len(rowNames))
	prefix := cfCommit + ":"

	err := util.ChunkIter(len(rowNames), getBatchSize, func(bStart, bEnd int) error {
		egroup.Go(func() error {
			bRowNames := rowNames[bStart:bEnd]
			batchIdx := int64(bStart - 1)
			err := b.table.ReadRows(ctx, bRowNames, func(row bigtable.Row) bool {
				longCommit := vcsinfo.NewLongCommit()
				longCommit.Hash = keyFromRowName(row.Key())

				for _, col := range row[cfCommit] {
					switch strings.TrimPrefix(col.Column, prefix) {
					case colHash:
						longCommit.Timestamp = col.Timestamp.Time().UTC()
					case colAuthor:
						longCommit.Author = string(col.Value)
					case colSubject:
						longCommit.Subject = string(col.Value)
					case colParents:
						if len(col.Value) > 0 {
							longCommit.Parents = strings.Split(string(col.Value), ":")
						}
					case colBody:
						longCommit.Body = string(col.Value)
					case colBranches:
						if err := json.Unmarshal(col.Value, &longCommit.Branches); err != nil {
							// We don't want to fail forever if there's a bad value in
							// BigTable. Log an error and move on.
							sklog.Errorf("Failed to decode LongCommit branches: %s\nStored value: %s", err, string(col.Value))
						}
					case colIndex:
						index, err := strconv.Atoi(string(col.Value))
						if err != nil {
							// We don't want to fail forever if there's a bad value in
							// BigTable. Log an error and move on.
							sklog.Errorf("Failed to decode LongCommit branches: %s\nStored value: %s", err, string(col.Value))
						}
						longCommit.Index = index
					}
				}
				targetIdx := atomic.AddInt64(&batchIdx, 1)
				tempRet[targetIdx] = longCommit
				return true
			})
			if err != nil {
				return skerr.Fmt("Error running ReadRows: %s", err)
			}
			return nil
		})
		return nil
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to spin up goroutines to load commits.")
	}

	if err := egroup.Wait(); err != nil {
		return nil, skerr.Wrapf(err, "Failed loading commits from BT.")
	}

	// Order the LongCommits to match the passed-in slice of hashes.
	ret := make([]*vcsinfo.LongCommit, len(hashes))
	for _, commit := range tempRet {
		if commit != nil {
			for _, targetIdx := range hashOrder[commit.Hash] {
				ret[targetIdx] = commit
			}
		}
	}
	return ret, nil
}

// PutBranches implements the GitStore interface.
func (b *BigTableGitStore) PutBranches(ctx context.Context, branches map[string]string) error {
	// Get the commits pointed to by the branches.
	hashes := make([]string, 0, len(branches))
	for _, head := range branches {
		if head != gitstore.DELETE_BRANCH {
			hashes = append(hashes, head)
		}
	}
	longCommits, err := b.Get(ctx, hashes)
	if err != nil {
		return skerr.Wrapf(err, "Failed to retrieve branch heads.")
	}
	indexCommitsByHash := make(map[string]*vcsinfo.IndexCommit, len(longCommits))
	for idx, c := range longCommits {
		if c == nil {
			return skerr.Fmt("Commit %s is missing from GitStore", hashes[idx])
		}
		indexCommitsByHash[c.Hash] = &vcsinfo.IndexCommit{
			Hash:      c.Hash,
			Index:     c.Index,
			Timestamp: c.Timestamp,
		}
	}

	var egroup errgroup.Group
	for name, head := range branches {
		// https://golang.org/doc/faq#closures_and_goroutines
		name := name
		head := head
		egroup.Go(func() error {
			if head == gitstore.DELETE_BRANCH {
				return b.deleteBranchPointer(ctx, name)
			} else {
				return b.putBranchPointer(ctx, getRepoInfoRowName(b.RepoURL), name, indexCommitsByHash[head])
			}
		})
	}
	if err := egroup.Wait(); err != nil {
		return skerr.Fmt("Error updating branches: %s", err)
	}
	return nil
}

// GetBranches implements the GitStore interface.
func (b *BigTableGitStore) GetBranches(ctx context.Context) (map[string]*gitstore.BranchPointer, error) {
	repoInfo, err := b.loadRepoInfo(ctx, false)
	if err != nil {
		return nil, err
	}
	return repoInfo.Branches, nil
}

// RangeByTime implements the GitStore interface.
func (b *BigTableGitStore) RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error) {
	startTS := sortableTimestamp(start)
	endTS := sortableTimestamp(end)

	result := newSRTimestampCommits(b.shards)
	// Note that we do NOT use a LatestN filter here, because that would
	// result in incomplete results in the case of commits which have the
	// same timestamp. Git has a timestamp resolution of one second, which
	// makes this likely, especially in tests.
	filters := []bigtable.Filter{bigtable.FamilyFilter(cfTsCommit)}
	err := b.iterShardedRange(ctx, branch, typTimeStamp, startTS, endTS, filters, result)
	if err != nil {
		return nil, err
	}

	return result.Sorted(), nil
}

// RangeByTime implements the GitStore interface.
func (b *BigTableGitStore) RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error) {
	startIdx := sortableIndex(startIndex)
	endIdx := sortableIndex(endIndex)

	result := newSRIndexCommits(b.shards)
	filters := []bigtable.Filter{bigtable.FamilyFilter(cfCommit), bigtable.LatestNFilter(1)}
	err := b.iterShardedRange(ctx, branch, typIndex, startIdx, endIdx, filters, result)
	if err != nil {
		return nil, err
	}
	return result.Sorted(), nil
}

func (b *BigTableGitStore) loadRepoInfo(ctx context.Context, create bool) (*gitstore.RepoInfo, error) {
	// load repo info
	rowName := getRepoInfoRowName(b.RepoURL)
	row, err := b.table.ReadRow(ctx, rowName, bigtable.RowFilter(bigtable.LatestNFilter(1)))
	if err != nil {
		return nil, err
	}

	if row != nil {
		return extractRepoInfo(row)
	}

	// If we are not create new repo information, return an error.
	if !create {
		return nil, skerr.Fmt("Repo information for %s not found.", b.RepoURL)
	}

	// Get a new ID from the DB
	rmw := bigtable.NewReadModifyWrite()
	rmw.Increment(cfMeta, colMetaIDCounter, 1)
	row, err = b.table.ApplyReadModifyWrite(ctx, unshardedRowName(typMeta, metaVarIDCounter), rmw)
	if err != nil {
		return nil, err
	}

	// encID contains the big-endian encoded ID
	encID := []byte(rowMap(row).GetStr(cfMeta, colMetaIDCounter))
	id := int64(binary.BigEndian.Uint64(encID))
	mut := bigtable.NewMutation()
	mut.Set(cfMeta, colMetaID, bigtable.ServerTime, encID)
	if err := b.table.Apply(ctx, rowName, mut); err != nil {
		return nil, err
	}

	b.RepoID = id
	return &gitstore.RepoInfo{
		RepoURL:  b.RepoURL,
		ID:       id,
		Branches: map[string]*gitstore.BranchPointer{},
	}, nil
}

// putBranchPointer writes the branch pointer (the HEAD of a branch) to the row that stores
// the repo information. idxCommit is the index commit of the HEAD of the branch.
func (b *BigTableGitStore) putBranchPointer(ctx context.Context, repoInfoRowName, branchName string, idxCommit *vcsinfo.IndexCommit) error {
	mut := bigtable.NewMutation()
	now := bigtable.Now()
	mut.Set(cfBranches, branchName, now, encBranchPointer(idxCommit.Hash, idxCommit.Index))
	mut.DeleteTimestampRange(cfBranches, branchName, 0, now)
	return b.table.Apply(ctx, repoInfoRowName, mut)
}

// deleteBranchPointer deletes the row containing the branch pointer.
func (b *BigTableGitStore) deleteBranchPointer(ctx context.Context, branchName string) error {
	mut := bigtable.NewMutation()
	mut.DeleteCellsInColumn(cfBranches, branchName)
	return b.table.Apply(ctx, getRepoInfoRowName(b.RepoURL), mut)
}

// writeLongCommits writes the LongCommits to the store idempotently.
func (b *BigTableGitStore) writeLongCommits(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	// Assemble the mutations.
	nMutations := len(commits)
	rowNames := make([]string, 0, nMutations)
	mutations := make([]*bigtable.Mutation, 0, nMutations)

	// Assemble the records for the Timestamp index.
	tsIdxCommits := make([]*vcsinfo.IndexCommit, 0, nMutations)

	for _, commit := range commits {
		// Add the long commits
		rowNames = append(rowNames, b.rowName("", typCommit, commit.Hash))
		mut, err := b.getCommitMutation(commit)
		if err != nil {
			return skerr.Wrapf(err, "Failed to create BT mutation")
		}
		mutations = append(mutations, mut)

		tsIdxCommits = append(tsIdxCommits, &vcsinfo.IndexCommit{
			Hash:      commit.Hash,
			Timestamp: commit.Timestamp,
		})
	}

	if err := b.applyBulkBatched(ctx, rowNames, mutations, writeBatchSize); err != nil {
		return skerr.Fmt("Error writing commits: %s", err)
	}
	return nil
}

// applyBulkBatched writes the given rowNames/mutation pairs to bigtable in batches that are
// maximally of size 'batchSize'. The batches are written in parallel.
func (b *BigTableGitStore) applyBulkBatched(ctx context.Context, rowNames []string, mutations []*bigtable.Mutation, batchSize int) error {
	var egroup errgroup.Group
	err := util.ChunkIter(len(rowNames), batchSize, func(chunkStart, chunkEnd int) error {
		egroup.Go(func() error {
			rowNames := rowNames[chunkStart:chunkEnd]
			mutations := mutations[chunkStart:chunkEnd]
			errs, err := b.table.ApplyBulk(ctx, rowNames, mutations)
			if err != nil {
				return skerr.Fmt("Error writing batch [%d:%d]: %s", chunkStart, chunkEnd, err)
			}
			if errs != nil {
				return skerr.Fmt("Error writing some portions of batch [%d:%d]: %s", chunkStart, chunkEnd, errs)
			}
			return nil
		})
		return nil
	})
	if err != nil {
		return skerr.Fmt("Error running ChunkIter: %s", err)
	}
	return egroup.Wait()
}

// writeIndexCommits writes the given index commits keyed by their indices for the given branch.
func (b *BigTableGitStore) writeIndexCommits(ctx context.Context, indexCommits []*vcsinfo.IndexCommit, branch string) error {
	idxRowNames := make([]string, 0, len(indexCommits))
	idxMutations := make([]*bigtable.Mutation, 0, len(indexCommits))

	for idx, commit := range indexCommits {
		sIndex := sortableIndex(indexCommits[idx].Index)
		idxRowNames = append(idxRowNames, b.rowName(branch, typIndex, sIndex))
		idxMutations = append(idxMutations, b.simpleMutation(cfCommit, commit.Timestamp, [2]string{colHash, commit.Hash}))
	}

	if err := b.applyBulkBatched(ctx, idxRowNames, idxMutations, writeBatchSize); err != nil {
		return skerr.Fmt("Error writing indices: %s", err)
	}
	return nil
}

// writeTimestampIndex writes the given index commits keyed by their timestamp for the
// given branch.
func (b *BigTableGitStore) writeTimestampIndex(ctx context.Context, indexCommits []*vcsinfo.IndexCommit, branch string) error {
	nMutations := len(indexCommits)
	tsRowNames := make([]string, 0, nMutations)
	tsMutations := make([]*bigtable.Mutation, 0, nMutations)

	for _, commit := range indexCommits {
		tsRowName := b.rowName(branch, typTimeStamp, sortableTimestamp(commit.Timestamp))
		tsRowNames = append(tsRowNames, tsRowName)
		tsMutations = append(tsMutations, b.simpleMutation(cfTsCommit, commit.Timestamp, [][2]string{
			{commit.Hash, sortableIndex(commit.Index)},
		}...))
	}

	// Write the timestamped index.
	if err := b.applyBulkBatched(ctx, tsRowNames, tsMutations, writeBatchSize); err != nil {
		return skerr.Fmt("Error writing timestamps: %s", err)
	}
	return nil
}

// iterShardedRange iterates the keys in the half open interval [startKey, endKey) across all
// shards triggering as many queries as there are shards. If endKey is empty, then startKey is
// used to generate a prefix and a Prefix scan is performed.
// The results of the query are added to the instance of shardedResults.
func (b *BigTableGitStore) iterShardedRange(ctx context.Context, branch, rowType, startKey, endKey string, filters []bigtable.Filter, result shardedResults) error {
	var egroup errgroup.Group

	// Query all shards in parallel.
	for shard := uint32(0); shard < b.shards; shard++ {
		// https://golang.org/doc/faq#closures_and_goroutines
		shard := shard
		egroup.Go(func() error {
			defer result.Finish(shard)

			var rr bigtable.RowRange
			// Treat the startKey as part of a prefix and do a prefix scan.
			if endKey == "" {
				rowPrefix := b.shardedRowName(shard, branch, rowType, startKey)
				rr = bigtable.PrefixRange(rowPrefix)
			} else {
				// Derive the start and end row names.
				rStart := b.shardedRowName(shard, branch, rowType, startKey)
				rEnd := b.shardedRowName(shard, branch, rowType, endKey)
				rr = bigtable.NewRange(rStart, rEnd)
			}

			var addErr error
			err := b.table.ReadRows(ctx, rr, func(row bigtable.Row) bool {
				addErr = result.Add(shard, row)
				return addErr == nil
			}, filtersToReadOptions(filters)...)
			if err != nil {
				return err
			}
			return addErr
		})
	}

	if err := egroup.Wait(); err != nil {
		return err
	}
	return nil
}

// simpleMutation assembles a simple mutation consisting of a column family, a timestamp and a
// set of column/value pairs. The timestamp is applied to all column/pairs.
func (b *BigTableGitStore) simpleMutation(cfFam string, timeStamp time.Time, colValPairs ...[2]string) *bigtable.Mutation {
	ts := bigtable.Time(timeStamp.UTC())
	ret := bigtable.NewMutation()
	for _, pair := range colValPairs {
		ret.Set(cfFam, pair[0], ts, []byte(pair[1]))
	}
	return ret
}

// getCommitMutation gets the mutation to write a long commit. Since the timestamp is set to the
// timestamp of the commit this is idempotent.
func (b *BigTableGitStore) getCommitMutation(commit *vcsinfo.LongCommit) (*bigtable.Mutation, error) {
	ts := bigtable.Time(commit.Timestamp.UTC())
	ret := bigtable.NewMutation()
	ret.Set(cfCommit, colHash, ts, []byte(commit.Hash))
	ret.Set(cfCommit, colAuthor, ts, []byte(commit.Author))
	ret.Set(cfCommit, colSubject, ts, []byte(commit.Subject))
	ret.Set(cfCommit, colParents, ts, []byte(strings.Join(commit.Parents, ":")))
	ret.Set(cfCommit, colBody, ts, []byte(commit.Body))
	encBranches, err := json.Marshal(commit.Branches)
	if err != nil {
		return nil, err
	}
	ret.Set(cfCommit, colBranches, ts, encBranches)
	ret.Set(cfCommit, colIndex, ts, []byte(strconv.Itoa(commit.Index)))
	return ret, nil
}

// rowName returns that BT rowName based on the tuple: (branch,rowType,Key).
// It also derives a unique shard for the given tuple and generates the complete rowName.
func (b *BigTableGitStore) rowName(branch string, rowType string, key string) string {
	return b.shardedRowName(crc32.ChecksumIEEE([]byte(key))%b.shards, branch, rowType, key)
}

// shardedRowName returns the row name from (shard, branch, rowType, key) this is useful
// when we want to generate a specific row name with a defined shard.
func (b *BigTableGitStore) shardedRowName(shard uint32, branch, rowType, key string) string {
	return fmt.Sprintf("%02d:%04d:%s:%s:%s", shard, b.RepoID, branch, rowType, key)
}

// unshardedRowName concatenates parts without prefixing any sharding information. This is for
// row types that are not sharded.
func unshardedRowName(parts ...string) string {
	return strings.Join(parts, ":")
}

// getRepoInfoRowName returns the name of the row where the repo meta data is stored based on the repoURL.
func getRepoInfoRowName(repoURL string) string {
	return unshardedRowName(typMeta, metaVarRepo, repoURL)
}

// getRepoInfoRowNamePrefix returns the row Name prefix to scan all rows containing repo meta data.
func getRepoInfoRowNamePrefix() string {
	return unshardedRowName(typMeta, metaVarRepo)
}

// extractRepoInfo extract the repo meta data information from a read row.
func extractRepoInfo(row bigtable.Row) (*gitstore.RepoInfo, error) {
	rm := rowMap(row)

	// Extract the branch info.
	branchInfo := rm.GetStrMap(cfBranches)
	branches := make(map[string]*gitstore.BranchPointer, len(branchInfo))
	var err error
	for name, b := range branchInfo {
		branches[name], err = decBranchPointer([]byte(b))
		if err != nil {
			return nil, skerr.Fmt("Error decoding branch pointer: %s", err)
		}
	}

	// Extract the repo ID.
	idBytes := []byte(rm.GetStr(cfMeta, colMetaID))
	if len(idBytes) != 8 {
		return nil, skerr.Fmt("Error: Got id that's not exactly 8 bytes: '%x': %s", idBytes, err)
	}

	ret := &gitstore.RepoInfo{
		RepoURL:  keyFromRowName(row.Key()),
		ID:       int64(binary.BigEndian.Uint64(idBytes)),
		Branches: branches,
	}
	return ret, nil
}

// filtersToReadOptions converts a list of filters to []bigtable.ReadOption. It will inject
// a ChainFilters(...) instance if multiple filters are defined. By returning a slice we are
// able to pass 0 - n filter to a ReadRows call.
func filtersToReadOptions(filters []bigtable.Filter) []bigtable.ReadOption {
	if len(filters) == 0 {
		return []bigtable.ReadOption{}
	}

	// If there is more than one filter then chain them.
	if len(filters) > 1 {
		filters = []bigtable.Filter{bigtable.ChainFilters(filters...)}
	}

	return []bigtable.ReadOption{bigtable.RowFilter(filters[0])}
}

// keyFromRowName assumes that key segments are separated by ':' and the last segment is the
// actual key we are interested in.
func keyFromRowName(rowName string) string {
	parts := strings.Split(rowName, ":")
	return parts[len(parts)-1]
}

// sortableTimestamp returns a timestamp as a string (in seconds) that can used as a key in BT.
func sortableTimestamp(ts time.Time) string {
	// Convert the timestamp in seconds to a string that is sortable and limit it to 10 digits.
	// That is the equivalent of valid timestamps up to November 2286.
	return fmt.Sprintf("%010d", util.MinInt64(9999999999, ts.Unix()))
}

// sortableIndex returns an index as a string that can be used as a key in BT.
func sortableIndex(index int) string {
	return fmt.Sprintf("%08d", util.MinInt(99999999, index))
}

// parseIndex parses an index previously generated with sortableIndex.
func parseIndex(indexStr string) int {
	ret, err := strconv.ParseInt(indexStr, 10, 64)
	if err != nil {
		return -1
	}
	return int(ret)
}

// encBranchPointer converts a hash and an index into a string where the parts are separated by ':'
func encBranchPointer(hash string, index int) []byte {
	idxBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(idxBuf, uint64(index))
	return []byte(hash + ":" + string(idxBuf))
}

// decBranchPointer a string previously generated by encBranchPointer into a BranchPointer,
// containing a head and index.
func decBranchPointer(encPointer []byte) (*gitstore.BranchPointer, error) {
	parts := bytes.SplitN(encPointer, []byte(":"), 2)
	if len(parts) != 2 || len(parts[1]) != 8 {
		return nil, skerr.Fmt("Received wrong branch pointer. Expected format <commit>:<big_endian_64_bit>")
	}
	return &gitstore.BranchPointer{
		Head:  string(parts[0]),
		Index: int(binary.BigEndian.Uint64([]byte(parts[1]))),
	}, nil
}

// rowMap is a helper type that wraps around a bigtable.Row and allows to extract columns and their
// values.
type rowMap bigtable.Row

// GetStr extracts that value of colFam:colName as a string from the row. If it doesn't exist it
// returns ""
func (r rowMap) GetStr(colFamName, colName string) string {
	prefix := colFamName + ":"
	for _, col := range r[colFamName] {
		if strings.TrimPrefix(col.Column, prefix) == colName {
			return string(col.Value)
		}
	}
	return ""
}

// GetStrMap extracts a map[string]string from the row that maps columns -> values for the given
// column family.
func (r rowMap) GetStrMap(colFamName string) map[string]string {
	prefix := colFamName + ":"
	ret := make(map[string]string, len(r[colFamName]))
	for _, col := range r[colFamName] {
		trimmed := strings.TrimPrefix(col.Column, prefix)
		ret[trimmed] = string(col.Value)
	}
	return ret
}

// Make sure BigTableGitStore fulfills the GitStore interface
var _ gitstore.GitStore = (*BigTableGitStore)(nil)
