package tiling

import (
	"net/url"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// FillType is how filling in of missing values should be done in Trace.Grow().
type FillType int

const (
	FILL_BEFORE FillType = iota
	FILL_AFTER
)

// Trace represents a single series of measurements. The actual values it
// stores per Commit is defined by implementations of Trace.
type Trace interface {
	// Params returns the parameters (key-value pairs) that describe this trace.
	// For example, os:Android, gpu:nVidia
	Params() map[string]string

	// Merge this trace with the given trace. The given trace is expected to come
	// after this trace.
	Merge(Trace) Trace

	DeepCopy() Trace

	// Grow the measurements, filling in with sentinel values either before or
	// after based on FillType.
	Grow(int, FillType)

	// Len returns the number of samples in the series.
	Len() int

	// IsMissing returns true if the measurement at index i is a sentinel value,
	// for example, config.MISSING_DATA_SENTINEL.
	IsMissing(i int) bool

	// Trim trims the measurements to just the range from [begin, end).
	//
	// Just like a Go [:] slice this is inclusive of begin and exclusive of end.
	// The length on the Trace will then become end-begin.
	Trim(begin, end int) error
}

// TraceID helps document when strings should represent TraceIds
type TraceID string

// Matches returns true if the given Trace matches the given query.
func Matches(tr Trace, query url.Values) bool {
	for k, values := range query {
		if p, ok := tr.Params()[k]; !ok || !util.In(p, values) {
			return false
		}
	}
	return true
}

// Commit is information about each Git commit.
type Commit struct {
	// CommitTime is in seconds since the epoch
	CommitTime int64  `json:"commit_time" bq:"timestamp" db:"ts"`
	Hash       string `json:"hash"        bq:"gitHash"   db:"githash"`
	Author     string `json:"author"                     db:"author"`
}

// FindCommit searches the given commits for the given hash and returns the
// index of the commit and the commit itself. If the commit cannot be
// found (-1, nil) is returned.
func FindCommit(commits []*Commit, targetHash string) (int, *Commit) {
	if targetHash == "" {
		return -1, nil
	}
	for idx, commit := range commits {
		if commit.Hash == targetHash {
			return idx, commit
		}
	}
	return -1, nil
}

// Tile is a config.TILE_SIZE commit slice of data.
//
// The length of the Commits array is the same length as all of the Values
// arrays in all of the Traces.
type Tile struct {
	Traces   map[TraceID]Trace   `json:"traces"`
	ParamSet map[string][]string `json:"param_set"`
	Commits  []*Commit           `json:"commits"`

	// What is the scale of this Tile, i.e. it contains every Nth point, where
	// N=const.TILE_SCALE^Scale.
	Scale     int `json:"scale"`
	TileIndex int `json:"tileIndex"`
}

// LastCommitIndex returns the index of the last valid Commit.
func (t Tile) LastCommitIndex() int {
	for i := len(t.Commits) - 1; i > 0; i-- {
		if t.Commits[i].CommitTime != 0 {
			return i
		}
	}
	return 0
}

// CommitRange returns the hashes of the first and last commits in the Tile.
func (t Tile) CommitRange() (string, string) {
	return t.Commits[0].Hash, t.Commits[t.LastCommitIndex()].Hash
}

// Copy makes a copy of the tile where the Traces and Commits are deep copies and
// all the rest of the data is a shallow copy.
func (t Tile) Copy() *Tile {
	ret := &Tile{
		Traces:    map[TraceID]Trace{},
		ParamSet:  t.ParamSet,
		Scale:     t.Scale,
		TileIndex: t.TileIndex,
		Commits:   make([]*Commit, len(t.Commits)),
	}
	for i, c := range t.Commits {
		cp := *c
		ret.Commits[i] = &cp
	}
	for k, v := range t.Traces {
		ret.Traces[k] = v.DeepCopy()
	}
	return ret
}

// Trim trims the measurements to just the range from [begin, end).
//
// Just like a Go [:] slice this is inclusive of begin and exclusive of end.
// The length on the Traces will then become end-begin.
func (t Tile) Trim(begin, end int) (*Tile, error) {
	length := end - begin
	if end < begin || end > len(t.Commits) || begin < 0 {
		return nil, skerr.Fmt("Invalid Trim range [%d, %d) of [0, %d]", begin, end, length)
	}
	ret := &Tile{
		Traces:    map[TraceID]Trace{},
		ParamSet:  t.ParamSet,
		Scale:     t.Scale,
		TileIndex: t.TileIndex,
		Commits:   make([]*Commit, length),
	}

	for i := 0; i < length; i++ {
		cp := *t.Commits[i+begin]
		ret.Commits[i] = &cp
	}
	for k, v := range t.Traces {
		t := v.DeepCopy()
		if err := t.Trim(begin, end); err != nil {
			return nil, skerr.Wrapf(err, "trimming trace %s", k)
		}
		ret.Traces[k] = t
	}
	return ret, nil
}

// GetParamSet finds the paramSet for the given slice of traces.
func GetParamSet(traces map[TraceID]Trace, paramSet map[string][]string) {
	for _, trace := range traces {
		for k, v := range trace.Params() {
			if _, ok := paramSet[k]; !ok {
				paramSet[k] = []string{v}
			} else if !util.In(v, paramSet[k]) {
				paramSet[k] = append(paramSet[k], v)
			}
		}
	}
}

// Merge the two Tiles, presuming tile1 comes before tile2.
func Merge(tile1, tile2 *Tile) *Tile {
	n := len(tile1.Commits) + len(tile2.Commits)
	n1 := len(tile1.Commits)
	t := &Tile{
		Traces:   make(map[TraceID]Trace),
		ParamSet: make(map[string][]string),
		Commits:  make([]*Commit, n, n),
	}
	for i := range t.Commits {
		t.Commits[i] = &Commit{}
	}

	// Merge the Commits.
	for i, c := range tile1.Commits {
		t.Commits[i] = c
	}
	for i, c := range tile2.Commits {
		t.Commits[n1+i] = c
	}

	// Merge the Traces.
	seen := map[TraceID]bool{}
	for key, trace := range tile1.Traces {
		seen[key] = true
		if trace2, ok := tile2.Traces[key]; ok {
			t.Traces[key] = trace.Merge(trace2)
		} else {
			cp := trace.DeepCopy()
			cp.Grow(n, FILL_AFTER)
			t.Traces[key] = cp
		}
	}
	// Now add in the traces that are only in tile2.
	for key, trace := range tile2.Traces {
		if _, ok := seen[key]; ok {
			continue
		}
		cp := trace.DeepCopy()
		cp.Grow(n, FILL_BEFORE)
		t.Traces[key] = cp
	}

	// Recreate the ParamSet.
	GetParamSet(t.Traces, t.ParamSet)

	t.Scale = tile1.Scale
	t.TileIndex = tile1.TileIndex

	return t
}
