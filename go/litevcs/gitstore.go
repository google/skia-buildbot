package litevcs

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/davecgh/go-spew/spew"
	multierror "github.com/hashicorp/go-multierror"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
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
	cfMeta     = "M"

	// meta data columns.
	colMetaID        = "metaID"
	colMetaIDCounter = "metaIDCounter"

	// Keys of meta data rows.
	metaVarRepo      = "repo"
	metaVarIDCounter = "idcounter"

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

	// Define the row types.
	typIndex     = "i"
	typTimeStamp = "z"
	typCommit    = "k"
	typMeta      = "!"

	// getBatchSize is the batchsize for the Get operation. Each call to bigtable is maximally
	// with this number of git hashes. This is a conservative number to stay within the 1M request
	// size limit.
	getBatchSize = 5000
)

const ()

var (
	minTime = time.Time{}
	maxTime = time.Unix(int64(^uint64(0)>>1), 0)
	maxInt  = int(^uint(0) >> 1)

	ColumnFamilies = []string{cfCommit}
)

func InitBT(conf *BTConfig) error {
	return bt.InitBigtable(conf.ProjectID, conf.InstanceID, conf.TableID, []string{
		cfCommit,
		cfMeta,
		cfBranches,
	})
}

type GitStore interface {
	GetBranches() ([]*git.Branch, error)
	PutBranches([]*git.Branch) error
	Put(ctx context.Context, commits []*vcsinfo.LongCommit) error
	Get(ctx context.Context, hash []string) ([]*vcsinfo.LongCommit, []int, error)
	RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error)
	RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error)
}

// TODO
func GetRepos(conf *BTConfig) ([]string, error) {
	return nil, nil
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

func NewBTGitStore(config *BTConfig, repoURL string) (GitStore, error) {
	// Create the client.
	client, err := bigtable.NewClient(context.TODO(), config.ProjectID, config.InstanceID)
	if err != nil {
		return nil, skerr.Fmt("Error creating bigtable client: %s", err)
	}

	repoURL, err = normalizeURL(repoURL)
	if err != nil {
		return nil, skerr.Fmt("Error normalizing URL %q: %s", repoURL, err)
	}

	ret := &btGitStore{
		table:   client.Open(config.TableID),
		shards:  uint32(config.Shards),
		repoURL: repoURL,
	}

	if _, err := ret.getRepoInfo(true); err != nil {
		return nil, skerr.Fmt("Error getting initial repo info: %s", err)
	}
	return ret, nil
}

func normalizeURL(inputURL string) (string, error) {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", err
	}
	path := "/" + strings.TrimPrefix(strings.TrimSuffix(parsedURL.Path, ".git"), "/")
	return parsedURL.Host + path, nil
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
		ret[trimmed] = string(col.Value)
	}
	return ret
}

func (b *btGitStore) getRepoInfo(create bool) (*tRepoInfo, error) {
	// load repo info
	ctx := context.TODO()
	rowName := b.unshardedRowName(typMeta, metaVarRepo, b.repoURL)
	row, err := b.table.ReadRow(ctx, rowName)
	if err != nil {
		return nil, err
	}

	if row != nil {
		sklog.Infof("ABC: %s", spew.Sdump(row))
		rm := rowMap(row)
		ret := &tRepoInfo{
			RepoURL:  b.repoURL,
			Branches: rm.GetStrMap(cfBranches),
		}

		// Extract the repo ID.
		idBytes := []byte(rm.GetStr(cfMeta, colMetaID))
		if len(idBytes) != 8 {
			return nil, skerr.Fmt("Error: Got id that's not exactly 8 bytes: '%x': %s", idBytes, err)
		}
		ret.ID = int64(binary.BigEndian.Uint64(idBytes))
		b.repoID = ret.ID
		return ret, nil
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
		Branches: map[string]string{},
	}, nil
}

type tRepoInfo struct {
	ID       int64
	RepoURL  string
	Branches map[string]string
}

// TODO
func (b *btGitStore) GetBranches() ([]*git.Branch, error) {
	repoInfo, err := b.getRepoInfo(false)
	if err != nil {
		return nil, err
	}

	ret := make([]*git.Branch, 0, len(repoInfo.Branches))
	for name, head := range repoInfo.Branches {
		ret = append(ret, &git.Branch{Name: name, Head: head})
	}
	return ret, nil
}

func (b *btGitStore) loadBranches() (map[string]*git.Branch, error) {
	return nil, nil
}

func (b *btGitStore) addBranch(branch *git.Branch, commitGraph [][]string) ([][]string, error) {
	var err error
	if commitGraph == nil {
		if commitGraph, err = b.loadCommitGraph(); err != nil {
			return nil, err
		}
	}

	return commitGraph, nil
}

func (b *btGitStore) updateBranch(branch *git.Branch) error {
	return nil
}

func (b *btGitStore) PutBranches(branches []*git.Branch) error {
	var commitGraph [][]string
	knownBranches, err := b.loadBranches()
	if err != nil {
		return err
	}

	for _, newBranch := range branches {
		if foundBranch, ok := knownBranches[newBranch.Name]; ok {
			if foundBranch.Head == newBranch.Head {
				continue
			}
			if err := b.updateBranch(newBranch); err != nil {
				return err
			}
			continue
		}

		commitGraph, err = b.addBranch(newBranch, commitGraph)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *btGitStore) getIndexCommits(ctx context.Context, commits []*vcsinfo.LongCommit) ([]vcsinfo.IndexCommit, error) {
	// Load the details and index of the first commit and derive the indices of all other commits.

	fetchHashes := append([]string{commits[0].Hash}, commits[0].Parents...)
	details, indices, err := b.Get(ctx, fetchHashes)
	if err != nil {
		return nil, err
	}

	// If the commit exists we have found the index.
	commitIdx := 0
	if details[0] != nil {
		commitIdx = indices[0]
	} else {
		pIndex := -1
		for i := 1; i < len(details); i++ {
			if details[i] != nil {
				pIndex = util.MaxInt(indices[i], pIndex)
			}
		}
		if pIndex >= 0 {
			indexCommits, err := b.RangeN(ctx, pIndex, math.MaxInt32, "")
			if err != nil {
				return nil, err
			}
			sklog.Infof("index Commits: %s\n--------------------------------------\n\n", spew.Sdump(indexCommits))
			pIndex = indexCommits[len(indexCommits)-1].Index
		}
		commitIdx = pIndex + 1
	}

	// if len(commits[0].Parents) > 0 {
	// 	sklog.Infof("Getting parents: %s", spew.Sdump(commits[0].Parents))
	// 	details, indices, err := b.Get(ctx, commits[0].Parents)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	sklog.Infof("PParents: %s\n\n\n%s", spew.Sdump(details), spew.Sdump(indices))

	// 	commitIdx = -1
	// 	for i, d := range details {
	// 		if d != nil {
	// 			commitIdx = util.MaxInt(indices[i], commitIdx)
	// 		}
	// 	}
	// 	commitIdx++
	// }

	ret := make([]vcsinfo.IndexCommit, len(commits))
	for idx, commit := range commits {
		ret[idx].Hash = commit.Hash
		ret[idx].Index = commitIdx
		ret[idx].Timestamp = commit.Timestamp
		commitIdx++
	}
	return ret, nil
}

func (b *btGitStore) Put(ctx context.Context, commits []*vcsinfo.LongCommit) error {
	indexCommits, err := b.getIndexCommits(ctx, commits)
	if err != nil {
		return err
	}

	// Assemble the mutations.
	nMutations := len(commits)
	rowNames := make([]string, 0, nMutations)
	mutations := make([]*bigtable.Mutation, 0, nMutations)

	tsRowNames := make([]string, 0, nMutations)
	tsMutations := make([]*bigtable.Mutation, 0, nMutations)

	idxRowNames := make([]string, 0, nMutations)
	idxMutations := make([]*bigtable.Mutation, 0, nMutations)

	for idx, commit := range commits {
		sIndex := searchableIndex(indexCommits[idx].Index)

		// Add the long commits
		rowNames = append(rowNames, b.rowName("", typCommit, commit.Hash))
		mutations = append(mutations, b.getCommitMutation(commit, sIndex))

		// Add the timestamps in order
		tsRowName := b.rowName("", typTimeStamp, uniqueTimestamp(commit.Timestamp, sIndex))
		tsRowNames = append(tsRowNames, tsRowName)
		tsMutations = append(tsMutations, b.simpleMutation(commit.Timestamp, [][2]string{
			{colHash, commit.Hash},
		}...))

		// Add the indices in order
		idxRowNames = append(idxRowNames, b.rowName("", typIndex, sIndex))
		idxMutations = append(idxMutations, b.simpleMutation(commit.Timestamp, [2]string{colHash, commit.Hash}))
	}

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

func (b *btGitStore) loadCommitGraph() ([][]string, error) {
	return nil, nil
}

// func (b *btGitStore) Put(ctx context.Context, commits []*vcsinfo.LongCommit) error {
// 	// Assemble the mutations.
// 	nMutations := len(commits)
// 	rowNames := make([]string, 0, nMutations)
// 	mutations := make([]*bigtable.Mutation, 0, nMutations)
// 	tsRowNames := make([]string, 0, nMutations)
// 	tsMutations := make([]*bigtable.Mutation, 0, nMutations)
// 	idxRowNames := make([]string, 0, nMutations)
// 	idxMutations := make([]*bigtable.Mutation, 0, nMutations)
// 	for idx, commit := range commits {
// 		sIndex := searchableIndex(commitIndices[idx])

// 		rowNames = append(rowNames, b.rowName(typCommit, commit.Hash))
// 		mutations = append(mutations, b.getCommitMutation(commit, sIndex))

// 		tsRowNames = append(tsRowNames, b.rowName(typTimeStamp, searchableTimestamp(commit.Timestamp)))
// 		tsMutations = append(tsMutations, b.simpleMutation(commit.Timestamp, [][2]string{
// 			{colHash, commit.Hash},
// 			{colIndex, sIndex},
// 		}...))

// 		idxRowNames = append(idxRowNames, b.rowName(typIndex, sIndex))
// 		idxMutations = append(idxMutations, b.simpleMutation(commit.Timestamp, [2]string{colHash, commit.Hash}))
// 	}

// 	errs, err := b.table.ApplyBulk(context.TODO(), rowNames, mutations)
// 	if err != nil {
// 		return skerr.Fmt("Error writing commits: %s", err)
// 	}
// 	if errs != nil {
// 		return skerr.Fmt("Error writing some commits: %s", errs)
// 	}

// 	errs, err = b.table.ApplyBulk(context.TODO(), tsRowNames, tsMutations)
// 	if err != nil {
// 		return skerr.Fmt("Error writing timestamps: %s", err)
// 	}
// 	if errs != nil {
// 		return skerr.Fmt("Error writing some timestamps: %s", errs)
// 	}

// 	errs, err = b.table.ApplyBulk(context.TODO(), idxRowNames, idxMutations)
// 	if err != nil {
// 		return skerr.Fmt("Error writing indices: %s", err)
// 	}
// 	if errs != nil {
// 		return skerr.Fmt("Error writing some indices: %s", errs)
// 	}
// 	return nil
// }

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
	sort.Slice(ret, func(i, j int) bool { return ret[i].Index < ret[j].Index })
	return ret
}

func (b *btGitStore) RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error) {
	startTS := searchableTimestamp(start)
	endTS := searchableTimestamp(end)

	result := newSRIndexCommits(b.shards)
	err := b.iterShardedRange(ctx, branch, typTimeStamp, startTS, endTS, colHash, result)
	if err != nil {
		return nil, err
	}
	sklog.Infof("Before sort:")
	return result.Sorted(), nil
}

func (b *btGitStore) RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error) {
	startTS := searchableIndex(startIndex)
	endTS := searchableIndex(endIndex)

	result := newSRIndexCommits(b.shards)
	err := b.iterShardedRange(ctx, branch, typIndex, startTS, endTS, "", result)
	if err != nil {
		return nil, err
	}
	return result.Sorted(), nil
}

func (b *btGitStore) AllRange(ctx context.Context) error {
	rr := bigtable.PrefixRange("5:" + typIndex)
	return b.table.ReadRows(ctx, rr, func(row bigtable.Row) bool {
		sklog.Infof("row: %s", row.Key())
		return true
	})
}

func (b *btGitStore) iterShardedRange(ctx context.Context, branch, rowType, startKey, endKey, colFilter string, result shardedResults) error {
	var egroup errgroup.Group

	// Set up the filter for the query
	filter := bigtable.FamilyFilter(cfCommit)
	if colFilter != "" {
		filter = bigtable.ChainFilters(filter, bigtable.ColumnFilter(colFilter))
	}
	rowFilters := bigtable.RowFilter(filter)

	for shard := uint32(0); shard < b.shards; shard++ {
		func(shard uint32) {
			egroup.Go(func() error {
				defer result.Finish(shard)

				rStart := b.shardedRowName(shard, branch, rowType, startKey)
				rEnd := b.shardedRowName(shard, branch, rowType, endKey)
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
	ts := bigtable.Time(timeStamp.UTC())
	ret := bigtable.NewMutation()
	for _, pair := range colValPairs {
		ret.Set(cfCommit, pair[0], ts, []byte(pair[1]))
	}
	return ret
}

func (b *btGitStore) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, []int, error) {
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
						case colIndex:
							commitIdx = parseIndex(string(col.Value))
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
		return nil, nil, err
	}

	// sort.Slice(tempRet, func(i, j int) bool {
	// 	return hashOrder[tempRet[i].commit.Hash] < hashOrder[tempRet[j].commit.Hash]
	// })

	// Put the results into their places.
	ret := make([]*vcsinfo.LongCommit, len(hashes))
	indices := make([]int, len(hashes))
	for _, ic := range tempRet {
		if ic.commit != nil {
			targetIdx := hashOrder[ic.commit.Hash]
			ret[targetIdx] = ic.commit
			indices[targetIdx] = ic.index
		}
	}
	return ret, indices, nil
}

// func (b *btGitStore) Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, []int, error) {
// 	rowNames := make(bigtable.RowList, len(hashes))
// 	hashOrder := make(map[string]int, len(hashes))
// 	for idx, h := range hashes {
// 		rowNames[idx] = b.rowName("", typCommit, h)
// 		hashOrder[h] = idx
// 	}

// 	var egroup errgroup.Group
// 	tempRet := make([]idxCommit, len(hashes))

// 	for batchStart := 0; batchStart < len(rowNames); batchStart += getBatchSize {
// 		func(bStart, bEnd int) {
// 			egroup.Go(func() error {
// 				bRowNames := rowNames[bStart:bEnd]
// 				batchIdx := bStart

// 				err := b.table.ReadRows(ctx, bRowNames, func(row bigtable.Row) bool {
// 					longCommit := vcsinfo.NewLongCommit()
// 					longCommit.Hash = extractKey(row.Key())
// 					commitIdx := -1

// 					for _, col := range row[cfCommit] {
// 						switch strings.TrimPrefix(col.Column, cfCommitPrefix) {
// 						case colAuthor:
// 							longCommit.Author = string(col.Value)
// 							longCommit.Timestamp = col.Timestamp.Time().UTC()
// 						case colSubject:
// 							longCommit.Subject = string(col.Value)
// 						case colParents:
// 							if len(col.Value) > 0 {
// 								longCommit.Parents = strings.Split(string(col.Value), ":")
// 							} else {
// 								longCommit.Parents = []string{}
// 							}
// 						case colBody:
// 							longCommit.Body = string(col.Value)
// 						case colIndex:
// 							commitIdx = parseIndex(string(col.Value))
// 						}
// 					}
// 					tempRet[batchIdx].index = commitIdx
// 					tempRet[batchIdx].commit = longCommit
// 					batchIdx++
// 					return true
// 				})
// 				if err != nil {
// 					return skerr.Fmt("Error running ReadRows: %s", err)
// 				}
// 				return nil
// 			})
// 		}(batchStart, util.MinInt(batchStart+getBatchSize, len(rowNames)))
// 	}

// 	if err := egroup.Wait(); err != nil {
// 		return nil, err
// 	}

// 	sort.Slice(tempRet, func(i, j int) bool {
// 		return hashOrder[tempRet[i].commit.Hash] < hashOrder[tempRet[j].commit.Hash]
// 	})

// 	ret := make([]*vcsinfo.LongCommit, len(hashes))
// 	for idx, ic := range tempRet {
// 		ret[idx] = ic.commit
// 	}
// 	return ret, nil
// }

// TODO: remove this if we don't need it anymore
type idxCommit struct {
	index  int
	commit *vcsinfo.LongCommit
}

func (b *btGitStore) getCommitMutation(commit *vcsinfo.LongCommit, searchableIndex string) *bigtable.Mutation {
	ts := bigtable.Time(commit.Timestamp.UTC())
	ret := bigtable.NewMutation()
	ret.Set(cfCommit, colHash, ts, []byte(commit.Hash))
	ret.Set(cfCommit, colAuthor, ts, []byte(commit.Author))
	ret.Set(cfCommit, colSubject, ts, []byte(commit.Subject))
	ret.Set(cfCommit, colParents, ts, []byte(strings.Join(commit.Parents, ":")))
	ret.Set(cfCommit, colBody, ts, []byte(commit.Body))
	ret.Set(cfCommit, colIndex, ts, []byte(searchableIndex))
	return ret
}

func (b *btGitStore) rowName(branch string, rowType string, key string) string {
	return b.shardedRowName(crc32.ChecksumIEEE([]byte(key))%b.shards, branch, rowType, key)
}

func (b *btGitStore) shardedRowName(shard uint32, branch, rowType, key string) string {
	return fmt.Sprintf("%02d:%04d:%s:%s:%s", shard, b.repoID, branch, rowType, key)
}

func keyFromRowName(rowName string) string {
	parts := strings.Split(rowName, ":")
	return parts[len(parts)-1]
}

func uniqueTimestamp(ts time.Time, sIndex string) string {
	return searchableTimestamp(ts) + ":" + sIndex
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
