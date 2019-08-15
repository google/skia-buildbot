package bt_gitstore

// The bt_gitstore package implements a way to store Git metadata in BigTable for faster retrieval
// than requiring a local checkout.

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/skerr"
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

	// shortcommit
	colAuthor   = "a"
	colSubject  = "s"
	colParents  = "p"
	colBody     = "b"
	colBranches = "br"

	// Define the row types.
	typIndex     = "i"
	typTimeStamp = "z"
	typCommit    = "k"
	typMeta      = "!"

	// allCommitsBranch is a pseudo branch name to index all commits in a repo.
	allCommitsBranch = "@all-commits"

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
	branch := ""

	if err := b.writeLongCommits(ctx, commits); err != nil {
		return skerr.Fmt("Error writing long commits: %s", err)
	}

	// Retrieve the commits in time chronological order and set the index.
	indexCommits, err := b.RangeByTime(ctx, vcsinfo.MinTime, vcsinfo.MaxTime, branch)
	if err != err {
		return skerr.Fmt("Error retrieving commits in order: %s", err)
	}

	for idx, idxCommit := range indexCommits {
		idxCommit.Index = idx
	}
	return b.writeIndexCommits(ctx, indexCommits, branch)
}

// Get implements the GitStore interface.
func (b *BigTableGitStore) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error) {
	rowNames := make(bigtable.RowList, len(hashes))
	hashOrder := make(map[string]int, len(hashes))
	for idx, h := range hashes {
		rowNames[idx] = b.rowName("", typCommit, h)
		hashOrder[h] = idx
	}

	var egroup errgroup.Group
	tempRet := make([]*vcsinfo.LongCommit, len(hashes))
	prefix := cfCommit + ":"

	for batchStart := 0; batchStart < len(rowNames); batchStart += getBatchSize {
		func(bStart, bEnd int) {
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
		}(batchStart, util.MinInt(batchStart+getBatchSize, len(rowNames)))
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	// Put the results into their places based of the order of the input hashes.
	ret := make([]*vcsinfo.LongCommit, len(hashes))
	for _, commit := range tempRet {
		if commit != nil {
			targetIdx := hashOrder[commit.Hash]
			ret[targetIdx] = commit
		}
	}
	return ret, nil
}

// PutBranches implements the GitStore interface.
func (b *BigTableGitStore) PutBranches(ctx context.Context, branches map[string]string) error {
	repoInfo, err := b.loadRepoInfo(ctx, false)
	if err != nil {
		return err
	}

	// Load the commit graph.
	graph, err := b.GetGraph(ctx)
	if err != nil {
		return skerr.Fmt("Error loading graph: %s", err)
	}

	// updateFromm maps branchName -> branch_pointer_to_old_head to capture the branches we  need to update
	// and whether the branch existed before this update (the value of the map is not nil).
	updateFrom := make(map[string]*gitstore.BranchPointer, len(branches))
	for branchName, head := range branches {
		// Assume we start out with a completely fresh branch
		var oldHeadPtr *gitstore.BranchPointer = nil
		if foundHeadPtr, ok := repoInfo.Branches[branchName]; ok {
			// We are already done and do not need to update this branch.
			if foundHeadPtr.Head == head {
				continue
			}

			oldHeadNode := graph.GetNode(foundHeadPtr.Head)
			if oldHeadNode == nil {
				return skerr.Fmt("Unable to find previous head commit %s in graph", foundHeadPtr.Head)
			}
			oldHeadPtr = foundHeadPtr
		}
		updateFrom[branchName] = oldHeadPtr
	}

	var egroup errgroup.Group
	for branchName, oldHeadPtr := range updateFrom {
		func(branchName string, oldHeadPtr *gitstore.BranchPointer) {
			egroup.Go(func() error {
				if branches[branchName] == gitstore.DELETE_BRANCH {
					return b.deleteBranchPointer(ctx, branchName)
				} else {
					return b.updateBranch(ctx, branchName, branches[branchName], oldHeadPtr, graph)
				}
			})
		}(branchName, oldHeadPtr)
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

	// Replace the pseudo branch for all commits with an empty branch name.
	if found, ok := repoInfo.Branches[allCommitsBranch]; ok {
		repoInfo.Branches[""] = found
		delete(repoInfo.Branches, allCommitsBranch)
	}

	return repoInfo.Branches, nil
}

// RangeByTime implements the GitStore interface.
func (b *BigTableGitStore) RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error) {
	startTS := sortableTimestamp(start)
	endTS := sortableTimestamp(end)

	result := newSRTimestampCommits(b.shards)
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
	filters := []bigtable.Filter{bigtable.FamilyFilter(cfCommit)}
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

// graphColFilter defines a filter (regex) that only keeps columns we need to build the commit graph.
// Used by GetGraph.
var graphColFilter = fmt.Sprintf("(%s)", strings.Join([]string{colHash, colParents}, "|"))

// GetGraph implements the GitStore interface.
func (b *BigTableGitStore) GetGraph(ctx context.Context) (*gitstore.CommitGraph, error) {
	result := newRawNodesResult(b.shards)
	filters := []bigtable.Filter{
		bigtable.FamilyFilter(cfCommit),
		bigtable.ColumnFilter(graphColFilter),
	}
	if err := b.iterShardedRange(ctx, "", typCommit, "", "", filters, result); err != nil {
		return nil, skerr.Fmt("Error getting sharded commits: %s", err)
	}
	rawGraph, timeStamps := result.Merge()
	return gitstore.BuildGraph(rawGraph, timeStamps), nil
}

func (b *BigTableGitStore) getAsIndexCommits(ctx context.Context, ancestors []*gitstore.Node, startIdx int) ([]*vcsinfo.IndexCommit, error) {
	ret := make([]*vcsinfo.IndexCommit, len(ancestors))
	for idx, commitNode := range ancestors {
		ret[idx] = &vcsinfo.IndexCommit{
			Index:     startIdx + idx,
			Hash:      commitNode.Hash,
			Timestamp: commitNode.Timestamp,
		}
	}
	return ret, nil
}

// updateBranch updates the indices for the named branch and stores the branch pointer. It
// calculates the branch based on the given commit graph.
// If there is no previous branch then oldBranchPtr should be nil.
func (b *BigTableGitStore) updateBranch(ctx context.Context, branchName, newBranchHead string, oldBranchPtr *gitstore.BranchPointer, graph *gitstore.CommitGraph) error {
	// Make sure the new head node is in branch.
	headNode := graph.GetNode(newBranchHead)
	if headNode == nil {
		return skerr.Fmt("Head commit %s not found in commit graph", newBranchHead)
	}

	// If we have not previous branch we set the corresponding values so the logic below still works.
	if oldBranchPtr == nil {
		oldBranchPtr = &gitstore.BranchPointer{Head: "", Index: 0}
	}

	branchNodes := graph.DecendantChain(oldBranchPtr.Head, newBranchHead)
	startIndex := 0

	// If the hash of the first Node matches the hash of the old branchpointer we need to adjust
	// the initial value of index.
	if branchNodes[0].Hash == oldBranchPtr.Head {
		startIndex = oldBranchPtr.Index
	}
	indexCommits, err := b.getAsIndexCommits(ctx, branchNodes, startIndex)
	if err != nil {
		return skerr.Fmt("Error getting index commits for branch %s: %s", branchName, err)
	}

	// Write the index commits.
	if err := b.writeIndexCommits(ctx, indexCommits, branchName); err != nil {
		return err
	}

	// Write the index commits of the branch sorted by timestamps.
	return b.writeTimestampIndex(ctx, indexCommits, branchName)
}

// putBranchPointer writes the branch pointer (the HEAD of a branch) to the row that stores
// the repo information. idxCommit is the index commit of the HEAD of the branch.
func (b *BigTableGitStore) putBranchPointer(ctx context.Context, repoInfoRowName, branchName string, idxCommit *vcsinfo.IndexCommit) error {
	if branchName == "" {
		branchName = allCommitsBranch
	}

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
	branch := ""

	// Assemble the mutations.
	nMutations := len(commits)
	rowNames := make([]string, 0, nMutations)
	mutations := make([]*bigtable.Mutation, 0, nMutations)

	// Assemble the records for the Timestamp index.
	tsIdxCommits := make([]*vcsinfo.IndexCommit, 0, nMutations)

	for _, commit := range commits {
		// Add the long commits
		rowNames = append(rowNames, b.rowName(branch, typCommit, commit.Hash))
		mutations = append(mutations, b.getCommitMutation(commit))

		tsIdxCommits = append(tsIdxCommits, &vcsinfo.IndexCommit{
			Hash:      commit.Hash,
			Timestamp: commit.Timestamp,
		})
	}

	if err := b.applyBulkBatched(ctx, rowNames, mutations, writeBatchSize); err != nil {
		return skerr.Fmt("Error writing commits: %s", err)
	}
	return b.writeTimestampIndex(ctx, tsIdxCommits, branch)
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
	return b.putBranchPointer(ctx, getRepoInfoRowName(b.RepoURL), branch, indexCommits[len(indexCommits)-1])
}

// writeTimestampIndexCommits writes the given index commits keyed by their timestamp for the
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
		func(shard uint32) {
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
		}(shard)
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
func (b *BigTableGitStore) getCommitMutation(commit *vcsinfo.LongCommit) *bigtable.Mutation {
	ts := bigtable.Time(commit.Timestamp.UTC())
	ret := bigtable.NewMutation()
	ret.Set(cfCommit, colHash, ts, []byte(commit.Hash))
	ret.Set(cfCommit, colAuthor, ts, []byte(commit.Author))
	ret.Set(cfCommit, colSubject, ts, []byte(commit.Subject))
	ret.Set(cfCommit, colParents, ts, []byte(strings.Join(commit.Parents, ":")))
	ret.Set(cfCommit, colBody, ts, []byte(commit.Body))
	return ret
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
