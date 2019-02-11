package litevcs

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"golang.org/x/sync/errgroup"
)

// Keys:
//
//
//
// Valid Git branch names: https://stackoverflow.com/questions/3651860/which-characters-are-illegal-within-a-branch-name

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
	allCommitsBranch = "@@@sudobranch"

	// getBatchSize is the batchsize for the Get operation. Each call to bigtable is maximally
	// with this number of git hashes. This is a conservative number to stay within the 1M request
	// size limit.
	getBatchSize = 5000
)

var (
	MinTime = time.Unix(0, 0)

	// MaxTime is the maximum time for which functions like After(...) still work.
	// See:	https://stackoverflow.com/questions/25065055/what-is-the-maximum-time-time-in-go/32620397#32620397
	MaxTime = time.Unix(1<<63-62135596801, 999999999)
)

type GitStore interface {
	Put(ctx context.Context, commits []*vcsinfo.LongCommit) error
	Get(ctx context.Context, hash []string) ([]*vcsinfo.LongCommit, error)
	PutBranches(ctx context.Context, branches map[string]string) error
	GetBranches(ctx context.Context) (map[string]*BranchPointer, error)
	RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error)
	RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error)
}

type BranchPointer struct {
	Head  string
	Index int
}

type tRepoInfo struct {
	ID       int64
	RepoURL  string
	Branches map[string]*BranchPointer
}

type BTConfig struct {
	ProjectID  string
	InstanceID string
	TableID    string
	Shards     int
}

type btGitStore struct {
	table   *bigtable.Table
	shards  uint32
	repoURL string
	repoID  int64
}

func InitBT(conf *BTConfig) error {
	return bt.InitBigtable(conf.ProjectID, conf.InstanceID, conf.TableID, []string{
		cfCommit,
		cfMeta,
		cfBranches,
		cfTsCommit,
	})
}

func NewBTGitStore(config *BTConfig, repoURL string) (GitStore, error) {
	// Create the client.
	client, err := bigtable.NewClient(context.TODO(), config.ProjectID, config.InstanceID)
	if err != nil {
		return nil, skerr.Fmt("Error creating bigtable client: %s", err)
	}

	repoURL, err = NormalizeURL(repoURL)
	if err != nil {
		return nil, skerr.Fmt("Error normalizing URL %q: %s", repoURL, err)
	}

	ret := &btGitStore{
		table:   client.Open(config.TableID),
		shards:  uint32(config.Shards),
		repoURL: repoURL,
	}

	ctx := context.TODO()
	repoInfo, err := ret.loadRepoInfo(ctx, true)
	if err != nil {
		return nil, skerr.Fmt("Error getting initial repo info: %s", err)
	}
	ret.repoID = repoInfo.ID
	return ret, nil
}

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
	// sklog.Infof("XXX: %d", len(indexCommits))

	for idx, idxCommit := range indexCommits {
		idxCommit.Index = idx
	}
	return b.writeIndexCommits(ctx, indexCommits, branch)
}

func (b *btGitStore) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error) {
	rowNames := make(bigtable.RowList, len(hashes))
	hashOrder := make(map[string]int, len(hashes))
	for idx, h := range hashes {
		rowNames[idx] = b.rowName("", typCommit, h)
		hashOrder[h] = idx
	}

	var egroup errgroup.Group
	tempRet := make([]idxCommit, len(hashes))
	prefix := cfCommit + ":"

	for batchStart := 0; batchStart < len(rowNames); batchStart += getBatchSize {
		func(bStart, bEnd int) {
			egroup.Go(func() error {
				bRowNames := rowNames[bStart:bEnd]
				batchIdx := int64(bStart - 1)

				err := b.table.ReadRows(ctx, bRowNames, func(row bigtable.Row) bool {
					longCommit := vcsinfo.NewLongCommit()
					longCommit.Hash = keyFromRowName(row.Key())
					commitIdx := -1

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
					tempRet[targetIdx].index = commitIdx
					tempRet[targetIdx].commit = longCommit
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

	// Put the results into their places.
	ret := make([]*vcsinfo.LongCommit, len(hashes))
	for _, ic := range tempRet {
		if ic.commit != nil {
			targetIdx := hashOrder[ic.commit.Hash]
			ret[targetIdx] = ic.commit
		}
	}
	return ret, nil
}

func (b *btGitStore) PutBranches(ctx context.Context, branches map[string]string) error {
	repoInfo, err := b.loadRepoInfo(ctx, false)
	if err != nil {
		return err
	}

	graph, err := b.loadCommitGraph(ctx)
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

// TODO
func (b *btGitStore) GetBranches(ctx context.Context) (map[string]*BranchPointer, error) {
	repoInfo, err := b.loadRepoInfo(ctx, false)
	if err != nil {
		return nil, err
	}
	return repoInfo.Branches, nil
}

func (b *btGitStore) RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error) {
	startTS := searchableTimestamp(start)
	endTS := searchableTimestamp(end)

	result := newSRTimestampCommits(b.shards)
	err := b.iterShardedRange(ctx, branch, typTimeStamp, startTS, endTS, cfTsCommit, "", result)
	if err != nil {
		return nil, err
	}

	return result.Sorted(), nil
}

func (b *btGitStore) rangeTimeByPrefix(ctx context.Context, prefix string, branch string) ([]*vcsinfo.IndexCommit, error) {
	result := newSRTimestampCommits(b.shards)
	err := b.iterShardedRange(ctx, branch, typTimeStamp, prefix, "", cfCommit, colHash, result)
	if err != nil {
		return nil, err
	}
	// sklog.Infof("Before sort:")
	return result.Sorted(), nil
}

func (b *btGitStore) RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error) {
	startIdx := searchableIndex(startIndex)
	endIdx := searchableIndex(endIndex)
	// sklog.Infof("RangeN (%s): %s -> %s", branch, startIdx, endIdx)

	result := newSRIndexCommits(b.shards)
	err := b.iterShardedRange(ctx, branch, typIndex, startIdx, endIdx, cfCommit, "", result)
	if err != nil {
		return nil, err
	}
	return result.Sorted(), nil
}

func (b *btGitStore) getRepoInfoRowName() string {
	return b.unshardedRowName(typMeta, metaVarRepo, b.repoURL)
}

func (b *btGitStore) loadRepoInfo(ctx context.Context, create bool) (*tRepoInfo, error) {
	// load repo info
	rowName := b.getRepoInfoRowName()
	row, err := b.table.ReadRow(ctx, rowName, bigtable.RowFilter(bigtable.LatestNFilter(1)))
	if err != nil {
		return nil, err
	}

	if row != nil {
		rm := rowMap(row)

		// Extract the branch info.
		branchInfo := rm.GetStrMap(cfBranches)
		branches := make(map[string]*BranchPointer, len(branchInfo))
		for name, b := range branchInfo {
			branches[name], err = decBranchPointer([]byte(b))
			if err != nil {
				return nil, skerr.Fmt("Error decoding branch pointer: %s", err)
			}
		}
		// 	parts := strings.SplitN(b, ":", 2)
		// 	if len(parts) != 2 && len(parts[1]) != 8 {
		// 		return "", nil, skerr.Fmt("Received wrong branch pointer. Expected format <commit>:<big_endian_64_bit>")
		// 	}
		// 	branches[name] = &BranchPointer{
		// 		Head:  parts[0],
		// 		Index: int(binary.BigEndian.Uint64([]byte(parts[1]))),
		// 	}
		// }

		// Extract the repo ID.
		idBytes := []byte(rm.GetStr(cfMeta, colMetaID))
		if len(idBytes) != 8 {
			return nil, skerr.Fmt("Error: Got id that's not exactly 8 bytes: '%x': %s", idBytes, err)
		}
		ret := &tRepoInfo{
			RepoURL:  b.repoURL,
			ID:       int64(binary.BigEndian.Uint64(idBytes)),
			Branches: branches,
		}
		return ret, nil
	}

	// If we are not create new repo information, return an error.
	if !create {
		return nil, skerr.Fmt("Repo information for %s not found.", b.repoURL)
	}

	// Get a new ID from the DB
	rmw := bigtable.NewReadModifyWrite()
	rmw.Increment(cfMeta, colMetaIDCounter, 1)
	row, err = b.table.ApplyReadModifyWrite(ctx, b.unshardedRowName(typMeta, metaVarIDCounter), rmw)
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
	return &tRepoInfo{
		RepoURL:  b.repoURL,
		ID:       id,
		Branches: map[string]*BranchPointer{},
	}, nil
}

var graphColFilter = fmt.Sprintf("(%s)", strings.Join([]string{colHash, colParents}, "|"))

func (b *btGitStore) loadCommitGraph(ctx context.Context) (*CommitGraph, error) {
	t := timer.New("LOAD commit graph")
	result := newRawNodesResult(b.shards)
	if err := b.iterShardedRange(ctx, "", typCommit, "", "", cfCommit, graphColFilter, result); err != nil {
		return nil, skerr.Fmt("Error getting sharded commits: %s", err)
	}

	rawGraph, timeStamps := result.Merge()
	t.Stop()

	// // Do a scan of all commits and their parents
	// prefix := b.rowName("", typCommit, "")
	// rr := bigtable.PrefixRange(prefix)
	// sklog.Infof("Prefix range: %s", prefix)

	// rowFilter := bigtable.RowFilter(bigtable.ColumnFilter(graphColFilter))
	// initialSize := 100000
	// rawGraph := make([][]string, 0, initialSize)
	// err := b.table.ReadRows(ctx, rr, func(row bigtable.Row) bool {
	// 	var commitHash string
	// 	var parents []string
	// 	for _, col := range row[cfCommit] {
	// 		switch strings.TrimPrefix(col.Column, prefix) {
	// 		case colHash:
	// 			commitHash = string(col.Value)
	// 		case colParents:
	// 			if len(col.Value) > 0 {
	// 				parents = strings.Split(string(col.Value), ":")
	// 			}
	// 		}
	// 	}
	// 	node := make([]string, 0, 1+len(parents))
	// 	node = append(node, commitHash)
	// 	node = append(node, parents...)
	// 	rawGraph = append(rawGraph, node)
	// 	return true
	// }, rowFilter)
	// if err != nil {
	// 	return nil, err
	// }

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

func (b *btGitStore) updateBranch(ctx context.Context, branchName, newBranchHead string, repoInfo *tRepoInfo, graph *CommitGraph) error {
	// Find the target commit.
	headNode := graph.GetNode(newBranchHead)
	if headNode == nil {
		return skerr.Fmt("Head commit %s not found in commit graph", newBranchHead)
	}

	hashes := headNode.BranchCommits()

	indexCommits, err := b.getAsIndexCommits(ctx, hashes)
	if err != nil {
		return skerr.Fmt("Error getting index commits for branch %s: %s", branchName, err)
	}

	if err := b.writeIndexCommits(ctx, indexCommits, branchName); err != nil {
		return err
	}

	return b.writeTimestampIndex(ctx, indexCommits, branchName)
}

func (b *btGitStore) putBranchPointer(ctx context.Context, repoInfoRowName, branchName string, idxCommit *vcsinfo.IndexCommit) error {
	if branchName == "" {
		branchName = allCommitsBranch
	}

	mut := bigtable.NewMutation()
	now := bigtable.Now()
	mut.Set(cfBranches, branchName, now, encBranchPointer(idxCommit.Hash, idxCommit.Index))
	mut.DeleteTimestampRange(cfBranches, branchName, 0, now)

	// sklog.Infof("B: %40s   %s   %10d", branchName, idxCommit.Hash, idxCommit.Index)
	return b.table.Apply(ctx, repoInfoRowName, mut)
}

func (b *btGitStore) writeLongCommitsNG(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	branch := ""
	// Assemble the mutations.
	nMutations := len(commits)
	rowNames := make([]string, 0, nMutations)
	mutations := make([]*bigtable.Mutation, 0, nMutations)

	tsRowNames := make([]string, 0, nMutations)
	tsMutations := make([]*bigtable.Mutation, 0, nMutations)

	for _, commit := range commits {
		// Add the long commits
		rowNames = append(rowNames, b.rowName(branch, typCommit, commit.Hash))
		mutations = append(mutations, b.getCommitMutation(commit))

		// Add the timestamps in order
		tsRowName := b.rowName(branch, typTimeStamp, uniqueTimestamp(commit.Timestamp, commit.Hash))
		tsRowNames = append(tsRowNames, tsRowName)
		tsMutations = append(tsMutations, b.simpleMutation(cfCommit, commit.Timestamp, [][2]string{
			{colHash, commit.Hash},
		}...))

		// sklog.Infof("Adding %s       %s", rowNames[len(rowNames)-1], tsRowName)
	}

	errs, err := b.table.ApplyBulk(context.TODO(), rowNames, mutations)
	if err != nil {
		return skerr.Fmt("Error writing commits: %s", err)
	}
	if errs != nil {
		return skerr.Fmt("Error writing some commits: %s", errs)
	}

	// Write the timestamped index.
	errs, err = b.table.ApplyBulk(context.TODO(), tsRowNames, tsMutations)
	if err != nil {
		return skerr.Fmt("Error writing timestamps: %s", err)
	}
	if errs != nil {
		return skerr.Fmt("Error writing some timestamps: %s", errs)
	}
	return nil
}

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

	if err := b.applyBulkBatched(ctx, rowNames, mutations, longCommitsBatchSize); err != nil {
		return skerr.Fmt("Error writing commits: %s", err)
	}
	return b.writeTimestampIndex(ctx, tsIdxCommits, branch)
}

const (
	longCommitsBatchSize  = 1000
	timestampsBatchSize   = 1000
	indexCommitsBatchSize = 1000
)

func (b *btGitStore) applyBulkBatched(ctx context.Context, rowNames []string, mutations []*bigtable.Mutation, batchSize int) error {
	var egroup errgroup.Group
	for batchStart := 0; batchStart < len(mutations); batchStart += batchSize {
		batchEnd := util.MinInt(batchStart+batchSize, len(mutations))
		func(rowNames []string, mutations []*bigtable.Mutation) {
			egroup.Go(func() error {
				errs, err := b.table.ApplyBulk(context.TODO(), rowNames, mutations)
				if err != nil {
					return skerr.Fmt("Error writing batch: %s", err)
				}
				if errs != nil {
					return skerr.Fmt("Error writing some portions of batch: %s", errs)
				}
				return nil
			})
		}(rowNames[batchStart:batchEnd], mutations[batchStart:batchEnd])
	}
	return egroup.Wait()
}

func (b *btGitStore) writeIndexCommits(ctx context.Context, indexCommits []*vcsinfo.IndexCommit, branch string) error {
	idxRowNames := make([]string, 0, len(indexCommits))
	idxMutations := make([]*bigtable.Mutation, 0, len(indexCommits))

	for idx, commit := range indexCommits {
		sIndex := searchableIndex(indexCommits[idx].Index)
		idxRowNames = append(idxRowNames, b.rowName(branch, typIndex, sIndex))
		idxMutations = append(idxMutations, b.simpleMutation(cfCommit, commit.Timestamp, [2]string{colHash, commit.Hash}))
	}

	if err := b.applyBulkBatched(ctx, idxRowNames, idxMutations, indexCommitsBatchSize); err != nil {
		return skerr.Fmt("Error writing indices: %s", err)
	}
	return b.putBranchPointer(ctx, b.getRepoInfoRowName(), branch, indexCommits[len(indexCommits)-1])
}

func (b *btGitStore) writeTimestampIndex(ctx context.Context, indexCommits []*vcsinfo.IndexCommit, branch string) error {
	nMutations := len(indexCommits)
	tsRowNames := make([]string, 0, nMutations)
	tsMutations := make([]*bigtable.Mutation, 0, nMutations)

	for _, commit := range indexCommits {
		tsRowName := b.rowName(branch, typTimeStamp, searchableTimestamp(commit.Timestamp))
		tsRowNames = append(tsRowNames, tsRowName)
		tsMutations = append(tsMutations, b.simpleMutation(cfTsCommit, commit.Timestamp, [][2]string{
			{commit.Hash, searchableIndex(commit.Index)},
		}...))
	}

	// Write the timestamped index.
	if err := b.applyBulkBatched(ctx, tsRowNames, tsMutations, timestampsBatchSize); err != nil {
		return skerr.Fmt("Error writing timestamps: %s", err)
	}
	return nil
}

func (b *btGitStore) iterShardedRange(ctx context.Context, branch, rowType, startKey, endKey, cfFam, colFilter string, result shardedResults) error {
	var egroup errgroup.Group

	// Set up the filter for the query
	filters := []bigtable.Filter{}
	if true {
		filters = append(filters, bigtable.FamilyFilter(cfFam))
	}

	if colFilter != "" {
		filters = append(filters, bigtable.ColumnFilter(colFilter))
	}

	if len(filters) > 1 {
		filters = []bigtable.Filter{bigtable.ChainFilters(filters...)}
	}

	rowFilter := []bigtable.ReadOption{}
	if len(filters) == 1 {
		rowFilter = append(rowFilter, bigtable.RowFilter(filters[0]))
	}

	for shard := uint32(0); shard < b.shards; shard++ {
		func(shard uint32) {
			egroup.Go(func() error {
				defer result.Finish(shard)

				var rr bigtable.RowRange
				if endKey == "" {
					rowPrefix := b.shardedRowName(shard, branch, rowType, startKey)
					rr = bigtable.PrefixRange(rowPrefix)
				} else {
					rStart := b.shardedRowName(shard, branch, rowType, startKey)
					rEnd := b.shardedRowName(shard, branch, rowType, endKey)
					rr = bigtable.NewRange(rStart, rEnd)
				}

				var addErr error
				err := b.table.ReadRows(ctx, rr, func(row bigtable.Row) bool {
					addErr = result.Add(shard, row)
					return addErr == nil
				}, rowFilter...)
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

func (b *btGitStore) simpleMutation(cfFam string, timeStamp time.Time, colValPairs ...[2]string) *bigtable.Mutation {
	ts := bigtable.Time(timeStamp.UTC())
	ret := bigtable.NewMutation()
	for _, pair := range colValPairs {
		ret.Set(cfFam, pair[0], ts, []byte(pair[1]))
	}
	return ret
}

// TODO: remove this if we don't need it anymore
type idxCommit struct {
	index  int
	commit *vcsinfo.LongCommit
}

func (b *btGitStore) getCommitMutation(commit *vcsinfo.LongCommit) *bigtable.Mutation {
	ts := bigtable.Time(commit.Timestamp.UTC())
	ret := bigtable.NewMutation()
	ret.Set(cfCommit, colHash, ts, []byte(commit.Hash))
	ret.Set(cfCommit, colAuthor, ts, []byte(commit.Author))
	ret.Set(cfCommit, colSubject, ts, []byte(commit.Subject))
	ret.Set(cfCommit, colParents, ts, []byte(strings.Join(commit.Parents, ":")))
	ret.Set(cfCommit, colBody, ts, []byte(commit.Body))
	// ret.Set(cfCommit, colIndex, ts, []byte(searchableIndex))
	return ret
}

func (b *btGitStore) rowName(branch string, rowType string, key string) string {
	return b.shardedRowName(crc32.ChecksumIEEE([]byte(key))%b.shards, branch, rowType, key)
}

func (b *btGitStore) shardedRowName(shard uint32, branch, rowType, key string) string {
	return fmt.Sprintf("%02d:%04d:%s:%s:%s", shard, b.repoID, branch, rowType, key)
}

func (b *btGitStore) unshardedRowName(parts ...string) string {
	return strings.Join(parts, ":")
}

type rowMap bigtable.Row

func (r rowMap) GetStr(colFamName, colName string) string {
	prefix := colFamName + ":"
	for _, col := range r[colFamName] {
		if strings.TrimPrefix(col.Column, prefix) == colName {
			return string(col.Value)
		}
	}
	return ""
}

func (r rowMap) GetStrMap(colFamName string) map[string]string {
	prefix := colFamName + ":"
	ret := make(map[string]string, len(r[colFamName]))
	for _, col := range r[colFamName] {
		trimmed := strings.TrimPrefix(col.Column, prefix)
		// sklog.Infof("---> %s  -->   %q     -->  %x", trimmed, string(col.Value), col.Value)
		ret[trimmed] = string(col.Value)
	}
	return ret
}

func keyFromRowName(rowName string) string {
	parts := strings.Split(rowName, ":")
	return parts[len(parts)-1]
}

func uniqueTimestamp(ts time.Time, additionalField string) string {
	return searchableTimestamp(ts) + ":" + additionalField
}

func keyFromUniqueTimestamp(rowName string) string {
	parts := strings.Split(rowName, ":")
	return strings.Join(parts[len(parts)-2:], ":")
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

func NormalizeURL(inputURL string) (string, error) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", err
	}
	path := "/" + strings.TrimPrefix(strings.TrimSuffix(parsedURL.Path, ".git"), "/")
	return parsedURL.Host + path, nil
}

func encBranchPointer(hash string, index int) []byte {
	idxBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(idxBuf, uint64(index))
	return []byte(hash + ":" + string(idxBuf))
}

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
