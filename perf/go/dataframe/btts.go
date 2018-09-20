package dataframe

import (
	"bytes"
	"context"
	"encoding/binary"
	"time"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/btts"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/constants"
	"go.skia.org/infra/perf/go/ptracestore"
)

// bttsDataFrameBuilder implements DataFrameBuilder using btts.
type bttsDataFrameBuilder struct {
	vcs   vcsinfo.VCS
	store btts.BigTableTraceStore
}

func NewDataFrameBuilderFromBTTS(vcs vcsinfo.VCS, store btts.BigTableTraceStore) DataFrameBuilder {
	return &bttsDataFrameBuilder{
		vcs:   vcs,
		store: store,
	}
}

// rangeImpl returns the slices of ColumnHeader and cid.CommitID that
// are needed by DataFrame and ptracestore.PTraceStore, respectively. The
// slices are populated from the given vcsinfo.IndexCommits.
//
// The value for 'skip', the number of commits skipped, is passed through to
// the return values.
func rangeImpl(resp []*vcsinfo.IndexCommit, skip int) ([]*ColumnHeader, []*cid.CommitID, int) {
	headers := []*ColumnHeader{}
	commits := []*cid.CommitID{}
	for _, r := range resp {
		commits = append(commits, &cid.CommitID{
			Offset: r.Index,
			Source: "master",
		})
		headers = append(headers, &ColumnHeader{
			Source:    "master",
			Offset:    int64(r.Index),
			Timestamp: r.Timestamp.Unix(),
		})
	}
	return headers, commits, skip
}

// lastN returns the slices of ColumnHeader and cid.CommitID that are
// needed by DataFrame and ptracestore.PTraceStore, respectively. The slices
// are for the last N commits in the repo.
//
// Returns 0 for 'skip', the number of commits skipped.
func lastN(vcs vcsinfo.VCS, n int) ([]*ColumnHeader, []*cid.CommitID, int) {
	return rangeImpl(vcs.LastNIndex(n), 0)
}

// getRange returns the slices of ColumnHeader and cid.CommitID that are
// needed by DataFrame and ptracestore.PTraceStore, respectively. The slices
// are for the commits that fall in the given time range [begin, end).
//
// If 'downsample' is true then the number of commits returned is limited
// to MAX_SAMPLE_SIZE.
//
// The value for 'skip', the number of commits skipped, is also returned.
func getRange(vcs vcsinfo.VCS, begin, end time.Time, downsample bool) ([]*ColumnHeader, []*cid.CommitID, int) {
	commits := vcs.Range(begin, end)
	skip := 0
	if downsample {
		commits, skip = DownSample(vcs.Range(begin, end), MAX_SAMPLE_SIZE)
	}
	return rangeImpl(commits, skip)
}

// loadMatches loads values into 'traceSet' that match the 'matches' from the
// tile in the BoltDB 'db'.  Only values at the offsets in 'idxmap' are
// actually loaded, and 'idxmap' determines where they are stored in the Trace.
func loadMatches(bdb *bolt.DB, idxmap map[int]int, matches KeyMatches, traceSet TraceSet, traceLen int) error {
	defer timer.New("loadMatches time").Stop()

	get := func(tx *bolt.Tx) error {
		defer timer.New("loadMatches TX time").Stop()
		bucket := tx.Bucket([]byte(TRACE_VALUES_BUCKET_NAME))
		if bucket == nil {
			// If the bucket doesn't exist then we've never written to this tile, it's not an error,
			// it just means it has no data.
			return nil
		}
		v := bucket.Cursor()
		value := traceValue{}
		// Loop over the entire bucket.
		for btraceid, rawValues := v.First(); btraceid != nil; btraceid, rawValues = v.Next() {
			// Does the trace id match the query?
			if !matches(string(btraceid)) {
				continue
			}
			// Get the trace.
			trace := traceSet[string(btraceid)]
			if trace == nil {
				// Don't make the copy until we know we are going to need it.
				traceid := string(dup(btraceid))
				traceSet[traceid] = NewTrace(traceLen)
				trace = traceSet[traceid]
			}

			// Decode all the [index, float32] pairs stored for the trace.
			buf := bytes.NewBuffer(rawValues)
			for {
				if err := binary.Read(buf, binary.LittleEndian, &value); err != nil {
					break
				}
				// Store the value in trace if the index appears in idxmap.
				if offset, ok := idxmap[int(value.Index)]; ok {
					trace[offset] = value.Value
					// Don't break, we want the last value for index.
				}
			}
		}
		return nil
	}

	return bdb.View(get)
}

type tileMap struct {
	commitID *cid.CommitID
	idxmap   map[int]int
}

// buildMapper transforms the slice of commitIDs passed to Match into a mapping
// from the location of the commit in the DB to the index for that commit in
// the Trace's returned from Match. I.e. it maps tiles to a map that says where
// each value stored in the tile trace needs to be copied into the destination
// Trace.
//
// For example, if given:
//
//	commitIDs := []*cid.CommitID{
//		&cid.CommitID{
//			Source: "master",
//			Offset: 49,
//		},
//		&cid.CommitID{
//			Source: "master",
//			Offset: 50,
//		},
//		&cid.CommitID{
//			Source: "master",
//			Offset: 51,
//		},
//	}
//
// This will return the following, presuming a tile size of 50:
//
//	map[string]*tileMap{
//		"master-000000.bdb": &tileMap{
//			commitID: &cid.CommitID{
//				Source: "master",
//				Offset: 49,
//			},
//			idxmap: map[int]int{
//				49: 0,
//			},
//		},
//		"master-000001.bdb": &tileMap{
//			commitID: &cid.CommitID{
//				Source: "master",
//				Offset: 50,
//			},
//			idxmap: map[int]int{
//				0: 1,
//				1: 2,
//			},
//		},
//	}
//
// The returned map is used when loading traces out of tiles.
func buildMapper(commitIDs []*cid.CommitID) map[string]*tileMap {
	mapper := map[string]*tileMap{}
	for targetIndex, commitID := range commitIDs {
		if tm, ok := mapper[commitID.Filename()]; !ok {
			mapper[commitID.Filename()] = &tileMap{
				commitID: commitID,
				idxmap:   map[int]int{commitID.Offset % constants.COMMITS_PER_TILE: targetIndex},
			}
		} else {
			tm.idxmap[commitID.Offset%constants.COMMITS_PER_TILE] = targetIndex
		}
	}
	return mapper
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) New(progress ptracestore.Progress) (*DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewN(progress ptracestore.Progress, n int) (*DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewFromQueryAndRange(begin, end time.Time, q *query.Query, progress ptracestore.Progress) (*DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewFromKeysAndRange(keys []string, begin, end time.Time, progress ptracestore.Progress) (*DataFrame, error) {
	return nil, nil
}

// See DataFrameBuilder.
func (b *bttsDataFrameBuilder) NewFromCommitIDsAndQuery(ctx context.Context, cids []*cid.CommitID, cidl *cid.CommitIDLookup, q *query.Query, progress ptracestore.Progress) (*DataFrame, error) {
	return nil, nil
}

// Validate that the concrete bttsDataFrameBuilder faithfully implements the DataFrameBuidler interface.
var _ DataFrameBuilder = (*bttsDataFrameBuilder)(nil)
