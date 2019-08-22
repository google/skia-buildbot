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
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitstore"
	deprecated "go.skia.org/infra/go/gitstore_deprecated/bt_gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/sync/errgroup"
)

// BigTableGitStore implements the GitStore interface based on BigTable.
type BigTableGitStore struct {
	RepoID          int64
	RepoURL         string
	shards          uint32
	writeGoroutines int
	table           *bigtable.Table
}

// New returns an instance of GitStore that uses BigTable as its backend storage.
// The given repoURL serves to identify the repository. Internally it is stored normalized
// via a call to git.NormalizeURL.
func New(ctx context.Context, config *BTConfig, repoURL string) (*BigTableGitStore, error) {
	if config.TableID == deprecated.DEPRECATED_TABLE_ID {
		return nil, skerr.Fmt("This implementation of BigTableGitStore cannot be used with deprecated table %q", deprecated.DEPRECATED_TABLE_ID)
	}
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

	writeGoroutines := config.WriteGoroutines
	if writeGoroutines <= 0 {
		writeGoroutines = DefaultWriteGoroutines
	}
	ret := &BigTableGitStore{
		table:           client.Open(config.TableID),
		shards:          uint32(shards),
		RepoURL:         repoURL,
		writeGoroutines: writeGoroutines,
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
	colHashTs = "ht"

	// long commit
	colAuthor   = "a"
	colSubject  = "s"
	colParents  = "p"
	colBody     = "b"
	colBranches = "br"
	colHash     = "h"
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

	// Default number of shards used, if not shards provided in BTConfig.
	DefaultShards = 32

	// DefaultWriteGoroutines defines the maximum number of goroutines
	// used to write to BigTable concurrently, if not provided in BTConfig.
	// This number was shown to keep memory usage reasonably low while still
	// providing decent throughput.
	DefaultWriteGoroutines = 100
)

// rowMutation is a mutation for a single BT row.
type rowMutation struct {
	row string
	mut *bigtable.Mutation
}

// Put implements the GitStore interface.
func (b *BigTableGitStore) Put(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	if len(commits) == 0 {
		return nil
	}

	// Spin up a goroutine to create mutations for commits.
	mutations := make(chan rowMutation, writeBatchSize)
	var egroup errgroup.Group
	egroup.Go(func() error {
		defer func() {
			close(mutations)
		}()
		// Create IndexCommits mutations for each branch for each commit.
		for i, c := range commits {
			// Validation.
			if c.Index == 0 && len(c.Parents) != 0 {
				return skerr.Fmt("Commit %s has index zero but has at least one parent. This cannot be correct.", c.Hash)
			}
			if len(c.Branches) == 0 {
				// TODO(borenet): Is there any way to check for this?
				sklog.Warningf("Commit %s has no branch information; this is valid if it is not on the first-parent ancestry chain of any branch.", c.Hash)
			}

			// LongCommit mutation.
			mut, err := b.mutationForLongCommit(c)
			if err != nil {
				return skerr.Wrapf(err, "Failed to Put commits; failed to create mutation.")
			}
			mutations <- mut

			// Create the IndexCommit.
			ic := &vcsinfo.IndexCommit{
				Hash:      c.Hash,
				Index:     c.Index,
				Timestamp: c.Timestamp,
			}
			mutations <- b.mutationForIndexCommit(gitstore.ALL_BRANCHES, ic)
			mutations <- b.mutationForTimestampCommit(gitstore.ALL_BRANCHES, ic)
			for branch := range c.Branches {
				if branch != gitstore.ALL_BRANCHES {
					mutations <- b.mutationForIndexCommit(branch, ic)
					mutations <- b.mutationForTimestampCommit(branch, ic)
				}
			}
			if i%1000 == 0 {
				sklog.Infof("Created mutations for %d of %d commits.", i+1, len(commits))
			}
		}
		return nil
	})

	// Spin up workers to write to BT.
	empty := ""
	var egroup2 errgroup.Group
	for i := 0; i < b.writeGoroutines; i++ {
		egroup2.Go(func() error {
			rows := make([]string, 0, writeBatchSize)
			muts := make([]*bigtable.Mutation, 0, writeBatchSize)
			for rowMut := range mutations {
				rows = append(rows, rowMut.row)
				muts = append(muts, rowMut.mut)
				if len(rows) == writeBatchSize {
					if err := b.applyBulk(ctx, rows, muts); err != nil {
						return skerr.Wrapf(err, "Failed to write commits.")
					}
					// Reuse the buffers. We need to clear
					// out the values so that the underlying
					// elements can be GC'd.
					for i := 0; i < len(rows); i++ {
						rows[i] = empty
						muts[i] = nil
					}
					rows = rows[:0]
					muts = muts[:0]
				}
			}
			if len(rows) > 0 {
				if err := b.applyBulk(ctx, rows, muts); err != nil {
					return skerr.Wrapf(err, "Failed to write commits.")
				}
			}
			return nil
		})
	}

	// Wait for the inserts to finish.
	insertErr := egroup2.Wait()
	if insertErr != nil {
		// We need to consume all of the mutations so that the first
		// goroutine can exit.
		for range mutations {
		}
	}
	generateErr := egroup.Wait()
	if insertErr != nil && generateErr != nil {
		return skerr.Wrapf(generateErr, "Failed to generate BT mutations, and failed to apply with: %s", insertErr)
	} else if insertErr != nil {
		return skerr.Wrapf(insertErr, "Failed to apply BT mutations.")
	} else if generateErr != nil {
		return skerr.Wrapf(generateErr, "Failed to generate BT mutations.")
	}

	// Write the ALL_BRANCHES branch pointer.
	lastCommit := commits[len(commits)-1]
	ic := &vcsinfo.IndexCommit{
		Hash:      lastCommit.Hash,
		Index:     lastCommit.Index,
		Timestamp: lastCommit.Timestamp,
	}
	return b.putBranchPointer(ctx, getRepoInfoRowName(b.RepoURL), gitstore.ALL_BRANCHES, ic)
}

// mutationForLongCommit returns a rowMutation for the given LongCommit.
func (b *BigTableGitStore) mutationForLongCommit(commit *vcsinfo.LongCommit) (rowMutation, error) {
	mut, err := b.getCommitMutation(commit)
	if err != nil {
		return rowMutation{}, skerr.Wrapf(err, "Failed to create BT mutation")
	}
	return rowMutation{
		row: b.rowName("", typCommit, commit.Hash),
		mut: mut,
	}, nil
}

// mutationForIndexCommit returns a rowMutation for the given IndexCommit.
func (b *BigTableGitStore) mutationForIndexCommit(branch string, commit *vcsinfo.IndexCommit) rowMutation {
	return rowMutation{
		row: b.rowName(branch, typIndex, sortableIndex(commit.Index)),
		mut: b.simpleMutation(cfCommit, [][2]string{
			{colHashTs, fmt.Sprintf("%s#%d", commit.Hash, commit.Timestamp.Unix())}, // Git has a timestamp resolution of 1s.
		}...),
	}
}

// mutationForTimestampCommit returns a rowMutation for the given IndexCommit
// keyed by timestamp.
func (b *BigTableGitStore) mutationForTimestampCommit(branch string, commit *vcsinfo.IndexCommit) rowMutation {
	return rowMutation{
		row: b.rowName(branch, typTimeStamp, sortableTimestamp(commit.Timestamp)),
		mut: b.simpleMutation(cfTsCommit, [][2]string{
			{commit.Hash, sortableIndex(commit.Index)},
		}...),
	}
}

// applyBulk is a helper function for b.table.ApplyBulk.
func (b *BigTableGitStore) applyBulk(ctx context.Context, rows []string, muts []*bigtable.Mutation) error {
	errs, err := b.table.ApplyBulk(ctx, rows, muts)
	if err != nil {
		return skerr.Fmt("Error writing batch: %s", err)
	}
	if errs != nil {
		return skerr.Fmt("Error writing some portions of batch: %s", errs)
	}
	return nil
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

	// If a branch was supplied, retrieve the pointer.
	var branchPtr *gitstore.BranchPointer
	var egroup errgroup.Group
	if branch != gitstore.ALL_BRANCHES {
		egroup.Go(func() error {
			branches, err := b.GetBranches(ctx)
			if err != nil {
				return err
			}
			branchPtr = branches[branch]
			return nil
		})
	}

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
	indexCommits, timestamps := result.Sorted()

	// Filter out results which do not belong on the given branch.
	if err := egroup.Wait(); err != nil {
		return nil, skerr.Wrapf(err, "Failed to retrieve branch pointer for %s", branch)
	}
	if branchPtr == nil && branch != gitstore.ALL_BRANCHES {
		// If we don't know about the requested branch, return nil even
		// if we found IndexCommits. This is correct behavior for
		// deleted branches, because we don't delete the IndexCommits.
		return nil, nil
	}
	if branchPtr != nil {
		filtered := make(map[int][]*vcsinfo.IndexCommit, len(indexCommits))
		for _, ic := range indexCommits {
			if ic.Index <= branchPtr.Index {
				filtered[ic.Index] = append(filtered[ic.Index], ic)
			}
		}
		indexCommits = make([]*vcsinfo.IndexCommit, 0, len(filtered))
		for idx := 0; idx <= branchPtr.Index; idx++ {
			commits, ok := filtered[idx]
			if !ok {
				return nil, skerr.Fmt("Missing index %d for branch %s.", idx, branch)
			}
			if len(commits) == 1 {
				indexCommits = append(indexCommits, commits[0])
			} else {
				sklog.Warningf("History was changed for branch %s. Deduplicating by last insertion into BT.", branch)
				var mostRecent *vcsinfo.IndexCommit
				for _, ic := range commits {
					if mostRecent == nil || timestamps[ic].After(timestamps[mostRecent]) {
						mostRecent = ic
					}
				}
				indexCommits = append(indexCommits, mostRecent)
			}
		}
	}

	return indexCommits, nil
}

// RangeN implements the GitStore interface.
func (b *BigTableGitStore) RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error) {
	startIdx := sortableIndex(startIndex)
	endIdx := sortableIndex(endIndex)

	result := newSRIndexCommits(b.shards)
	filters := []bigtable.Filter{bigtable.FamilyFilter(cfCommit), bigtable.LatestNFilter(1)}
	err := b.iterShardedRange(ctx, branch, typIndex, startIdx, endIdx, filters, result)
	if err != nil {
		return nil, err
	}
	rv, _ := result.Sorted()
	return rv, nil
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
func (b *BigTableGitStore) simpleMutation(cfFam string, colValPairs ...[2]string) *bigtable.Mutation {
	ret := bigtable.NewMutation()
	for _, pair := range colValPairs {
		ret.DeleteCellsInColumn(cfFam, pair[0])
		ret.Set(cfFam, pair[0], bigtable.ServerTime, []byte(pair[1]))
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
