package gitstore

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
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/sync/errgroup"
)

// The file implements a way to store Git metadata in BigTable for faster retrieval.

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
	// Minimum time (the epoch)
	MinTime = time.Unix(0, 0)

	// MaxTime is the maximum time for which functions like After(...) still work.
	// See:	https://stackoverflow.com/questions/25065055/what-is-the-maximum-time-time-in-go/32620397#32620397
	MaxTime = time.Unix(1<<63-62135596801, 999999999)

	// Default number of shards used, if not shards provided in BTConfig.
	DefaultShards = 32
)

// GitStore defines the functions of a data store for Git metadata (aka vcsinfo.LongCommit)
// Each GitStore instance relates to one repository that is defined in the constructor of the
// implementation.
type GitStore interface {
	// Put stores the given commits. They can be retrieved in order of timestamps by using
	// RangeByTime or RangeN (no topological ordering).
	Put(ctx context.Context, commits []*vcsinfo.LongCommit) error

	// Get retrieves the commits identified by 'hashes'. The return value will always have the length
	// of the input value and the results will line up by index. If a commit does not exist the
	// corresponding entry in the result is nil.
	// The function will only return an error if the retrieval operation (the I/O) fails, not
	// if the given hashes do not exist or are invalid.
	Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error)

	// PutBranches updates branches in the repository. It writes indices for the branches so they
	// can be retrieved via RangeN and RangeByTime. These are ordered in toplogical order with only
	// first-parents included.
	// 'branches' maps branchName -> commit_hash to indicate the head of a branch. The store then
	// calculates the commits of the branch and updates the indices accordingly.
	// If a branch exists it will be updated. It will not remove existing branches in the repo if they
	// are not listed in the 'branches' argument.
	PutBranches(ctx context.Context, branches map[string]string) error

	// GetBranches returns the current branches in the store. It maps[branchName]->BranchPointer.
	// A BranchPointer contains the HEAD commit and also the Index of the HEAD commit, which is
	// usually the total number of commits in the branch minus 1.
	GetBranches(ctx context.Context) (map[string]*BranchPointer, error)

	// RangeN returns all commits in the half open index range [startIndex, endIndex).
	// Thus not including endIndex. It returns the commits in the given branch sorted by Index
	// and the commits are topologically sorted only including first-parent commits.
	RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error)

	// RangeByTime returns all commits in the half open time range [startIndex, endIndex). Thus not
	// including commits at 'end' time.
	// Caveat: The returned results will match the requested range, but will be sorted by Index.
	// So if the timestamps within a commit are not in order they will be unordered in the results.
	RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error)

	// GetGraph returns the commit graph of the entire repository.
	GetGraph(ctx context.Context) (*CommitGraph, error)
}

// BranchPointer captures the HEAD of a branch and the index of that commit.
type BranchPointer struct {
	Head  string
	Index int
}

// RepoInfo contains information about one repo in the GitStore.
type RepoInfo struct {
	// Numeric id of the repo. This is unique within all repos in a BT table. This ID is uniquely
	// assigned whenever a new repo is added.
	ID int64

	// RepoURL contains the URL of the repo as returned by git.NormalizeURL(...).
	RepoURL string

	// Branches contain all the branches in the repo, mapping branch_name -> branch_pointer.
	Branches map[string]*BranchPointer
}

// BTConfig contains the BigTable configuration to define where the repo should be stored.
type BTConfig struct {
	ProjectID  string
	InstanceID string
	TableID    string
	Shards     int
}

// btGitStore implements the GitStore interface base on BigTable.
type btGitStore struct {
	table   *bigtable.Table
	shards  uint32
	repoURL string
	repoID  int64
}

// InitBT initializes the BT instance for the given configuration. It uses the default way
// to get auth information from the environment and must be called with an account that has
// admin rights.
func InitBT(conf *BTConfig) error {
	return bt.InitBigtable(conf.ProjectID, conf.InstanceID, conf.TableID, []string{
		cfCommit,
		cfMeta,
		cfBranches,
		cfTsCommit,
	})
}

// AllRepos returns a map of all repos contained in given BigTable project/instance/table.
// It returns map[normalized_URL]RepoInfo.
func AllRepos(ctx context.Context, conf *BTConfig) (map[string]*RepoInfo, error) {
	// Create the client.
	client, err := bigtable.NewClient(ctx, conf.ProjectID, conf.InstanceID)
	if err != nil {
		return nil, skerr.Fmt("Error creating bigtable client: %s", err)
	}

	table := client.Open(conf.TableID)
	rowNamesPrefix := getRepoInfoRowNamePrefix()
	ret := map[string]*RepoInfo{}
	var readRowErr error = nil
	err = table.ReadRows(ctx, bigtable.PrefixRange(rowNamesPrefix), func(row bigtable.Row) bool {
		if readRowErr != nil {
			return false
		}

		var repoInfo *RepoInfo
		repoInfo, readRowErr = extractRepoInfo(row)
		if readRowErr != nil {
			return false
		}
		// save the repo info and set the all-commits branch.
		ret[repoInfo.RepoURL] = repoInfo
		if found, ok := repoInfo.Branches[allCommitsBranch]; ok {
			repoInfo.Branches[""] = found
			delete(repoInfo.Branches, allCommitsBranch)
		}

		return true
	}, bigtable.RowFilter(bigtable.LatestNFilter(1)))

	if err != nil {
		return nil, skerr.Fmt("Error reading repo info: %s", err)
	}
	return ret, nil
}

// NewBTGitStore returns an instance of GitStore that uses BigTable as its backend storage.
// The given repoURL serves to identify the repository. Internally it is stored normalized
// via a call to git.NormalizeURL.
func NewBTGitStore(ctx context.Context, config *BTConfig, repoURL string) (GitStore, error) {
	// Create the client.
	client, err := bigtable.NewClient(ctx, config.ProjectID, config.InstanceID)
	if err != nil {
		return nil, skerr.Fmt("Error creating bigtable client: %s", err)
	}

	repoURL, err = git.NormalizeURL(repoURL)
	if err != nil {
		return nil, skerr.Fmt("Error normalizing URL %q: %s", repoURL, err)
	}

	shards := config.Shards
	if shards <= 0 {
		shards = DefaultShards
	}

	ret := &btGitStore{
		table:   client.Open(config.TableID),
		shards:  uint32(shards),
		repoURL: repoURL,
	}

	repoInfo, err := ret.loadRepoInfo(ctx, true)
	if err != nil {
		return nil, skerr.Fmt("Error getting initial repo info: %s", err)
	}
	ret.repoID = repoInfo.ID
	return ret, nil
}

// Put implements the GitStore interface.
func (b *btGitStore) Put(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	branch := ""

	if err := b.writeLongCommits(ctx, commits); err != nil {
		return skerr.Fmt("Error writing long commits: %s", err)
	}

	// Retrieve the commits in time chronological order and set the index.
	indexCommits, err := b.RangeByTime(ctx, MinTime, MaxTime, branch)
	if err != err {
		return skerr.Fmt("Error retrieving commits in order: %s", err)
	}

	for idx, idxCommit := range indexCommits {
		idxCommit.Index = idx
	}
	return b.writeIndexCommits(ctx, indexCommits, branch)
}

// Get implements the GitStore interface.
func (b *btGitStore) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error) {
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
							} else {
								longCommit.Parents = []string{}
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
func (b *btGitStore) PutBranches(ctx context.Context, branches map[string]string) error {
	repoInfo, err := b.loadRepoInfo(ctx, false)
	if err != nil {
		return err
	}

	graph, err := b.GetGraph(ctx)
	if err != nil {
		return skerr.Fmt("Error loading graph: %s", err)
	}

	var egroup errgroup.Group
	for branchName, branchHead := range branches {
		func(branchName, branchHead string) {
			egroup.Go(func() error {
				return b.updateBranch(ctx, branchName, branchHead, repoInfo, graph)
			})
		}(branchName, branchHead)
	}
	if err := egroup.Wait(); err != nil {
		return skerr.Fmt("Error updating branches: %s", err)
	}
	return nil
}

// GetBranches implements the GitStore interface.
func (b *btGitStore) GetBranches(ctx context.Context) (map[string]*BranchPointer, error) {
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
func (b *btGitStore) RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error) {
	startTS := sortableTimestamp(start)
	endTS := sortableTimestamp(end)

	result := newSRTimestampCommits(b.shards)
	err := b.iterShardedRange(ctx, branch, typTimeStamp, startTS, endTS, cfTsCommit, "", result)
	if err != nil {
		return nil, err
	}

	return result.Sorted(), nil
}

// RangeByTime implements the GitStore interface.
func (b *btGitStore) RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error) {
	startIdx := sortableIndex(startIndex)
	endIdx := sortableIndex(endIndex)

	result := newSRIndexCommits(b.shards)
	err := b.iterShardedRange(ctx, branch, typIndex, startIdx, endIdx, cfCommit, "", result)
	if err != nil {
		return nil, err
	}
	return result.Sorted(), nil
}

func (b *btGitStore) loadRepoInfo(ctx context.Context, create bool) (*RepoInfo, error) {
	// load repo info
	rowName := getRepoInfoRowName(b.repoURL)
	row, err := b.table.ReadRow(ctx, rowName, bigtable.RowFilter(bigtable.LatestNFilter(1)))
	if err != nil {
		return nil, err
	}

	if row != nil {
		return extractRepoInfo(row)
	}

	// If we are not create new repo information, return an error.
	if !create {
		return nil, skerr.Fmt("Repo information for %s not found.", b.repoURL)
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

	b.repoID = id
	return &RepoInfo{
		RepoURL:  b.repoURL,
		ID:       id,
		Branches: map[string]*BranchPointer{},
	}, nil
}

// graphColFilter defines a filter (regex) that only keeps columns we need to build the commit graph.
// Used by GetGraph.
var graphColFilter = fmt.Sprintf("(%s)", strings.Join([]string{colHash, colParents}, "|"))

// GetGraph implements the GitStore interface.
func (b *btGitStore) GetGraph(ctx context.Context) (*CommitGraph, error) {
	result := newRawNodesResult(b.shards)
	if err := b.iterShardedRange(ctx, "", typCommit, "", "", cfCommit, graphColFilter, result); err != nil {
		return nil, skerr.Fmt("Error getting sharded commits: %s", err)
	}

	rawGraph, timeStamps := result.Merge()
	return buildGraph(rawGraph, timeStamps), nil
}

func (b *btGitStore) getAsIndexCommits(ctx context.Context, hashes []string) ([]*vcsinfo.IndexCommit, error) {
	details, err := b.Get(ctx, hashes)
	if err != nil {
		return nil, err
	}

	ret := make([]*vcsinfo.IndexCommit, len(details))
	for idx, commit := range details {
		ret[idx] = &vcsinfo.IndexCommit{
			Index:     idx,
			Hash:      commit.Hash,
			Timestamp: commit.Timestamp,
		}
	}
	return ret, nil
}

// updateBranch updates the indices for the named branch and stores the branch pointer. It
// calculates the branch based on the given commit graph.
func (b *btGitStore) updateBranch(ctx context.Context, branchName, newBranchHead string, repoInfo *RepoInfo, graph *CommitGraph) error {
	// Find the target commit.
	headNode := graph.GetNode(newBranchHead)
	if headNode == nil {
		return skerr.Fmt("Head commit %s not found in commit graph", newBranchHead)
	}

	// Get all commits in the branch.
	hashes := headNode.BranchCommits()
	indexCommits, err := b.getAsIndexCommits(ctx, hashes)
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
func (b *btGitStore) putBranchPointer(ctx context.Context, repoInfoRowName, branchName string, idxCommit *vcsinfo.IndexCommit) error {
	if branchName == "" {
		branchName = allCommitsBranch
	}

	mut := bigtable.NewMutation()
	now := bigtable.Now()
	mut.Set(cfBranches, branchName, now, encBranchPointer(idxCommit.Hash, idxCommit.Index))
	mut.DeleteTimestampRange(cfBranches, branchName, 0, now)
	return b.table.Apply(ctx, repoInfoRowName, mut)
}

// writeLongCommits writes the LongCommits to the store idempotently.
func (b *btGitStore) writeLongCommits(ctx context.Context, commits []*vcsinfo.LongCommit) error {
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
func (b *btGitStore) applyBulkBatched(ctx context.Context, rowNames []string, mutations []*bigtable.Mutation, batchSize int) error {
	var egroup errgroup.Group
	err := util.ChunkIter(len(rowNames), batchSize, func(chunkStart, chunkEnd int) error {
		egroup.Go(func() error {
			rowNames := rowNames[chunkStart:chunkEnd]
			mutations := mutations[chunkStart:chunkEnd]
			errs, err := b.table.ApplyBulk(context.TODO(), rowNames, mutations)
			if err != nil {
				return skerr.Fmt("Error writing batch: %s", err)
			}
			if errs != nil {
				return skerr.Fmt("Error writing some portions of batch: %s", errs)
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
func (b *btGitStore) writeIndexCommits(ctx context.Context, indexCommits []*vcsinfo.IndexCommit, branch string) error {
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
	return b.putBranchPointer(ctx, getRepoInfoRowName(b.repoURL), branch, indexCommits[len(indexCommits)-1])
}

// writeTimestampIndexCommits writes the given index commits keyed by their timestamp for the
// given branch.
func (b *btGitStore) writeTimestampIndex(ctx context.Context, indexCommits []*vcsinfo.IndexCommit, branch string) error {
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
func (b *btGitStore) iterShardedRange(ctx context.Context, branch, rowType, startKey, endKey, cfFam, colFilter string, result shardedResults) error {
	var egroup errgroup.Group

	// Set up the filter for the query
	filters := []bigtable.Filter{bigtable.FamilyFilter(cfFam)}
	if colFilter != "" {
		filters = append(filters, bigtable.ColumnFilter(colFilter))
	}

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
func (b *btGitStore) simpleMutation(cfFam string, timeStamp time.Time, colValPairs ...[2]string) *bigtable.Mutation {
	ts := bigtable.Time(timeStamp.UTC())
	ret := bigtable.NewMutation()
	for _, pair := range colValPairs {
		ret.Set(cfFam, pair[0], ts, []byte(pair[1]))
	}
	return ret
}

// getCommitMutation gets the mutation to write a long commit. Since the timestamp is set to the
// timestamp of the commit this is idempotent.
func (b *btGitStore) getCommitMutation(commit *vcsinfo.LongCommit) *bigtable.Mutation {
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
func (b *btGitStore) rowName(branch string, rowType string, key string) string {
	return b.shardedRowName(crc32.ChecksumIEEE([]byte(key))%b.shards, branch, rowType, key)
}

// shardedRowName returns the row name from (shard, branch, rowType, key) this is useful
// when we want to generate a specific row name with a defined shard.
func (b *btGitStore) shardedRowName(shard uint32, branch, rowType, key string) string {
	return fmt.Sprintf("%02d:%04d:%s:%s:%s", shard, b.repoID, branch, rowType, key)
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
func extractRepoInfo(row bigtable.Row) (*RepoInfo, error) {
	rm := rowMap(row)

	// Extract the branch info.
	branchInfo := rm.GetStrMap(cfBranches)
	branches := make(map[string]*BranchPointer, len(branchInfo))
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

	ret := &RepoInfo{
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
func decBranchPointer(encPointer []byte) (*BranchPointer, error) {
	parts := bytes.SplitN(encPointer, []byte(":"), 2)
	if len(parts) != 2 && len(parts[1]) != 8 {
		return nil, skerr.Fmt("Received wrong branch pointer. Expected format <commit>:<big_endian_64_bit>")
	}
	return &BranchPointer{
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
