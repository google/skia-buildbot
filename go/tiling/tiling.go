package tiling

import (
	"net/url"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// FillType is how filling in of missing values should be done in Trace.Grow().
type FillType int

const (
	FILL_BEFORE FillType = iota
	FILL_AFTER
)

const (
	// TILE_SCALE The number of points to subsample when moving one level of scaling. I.e.
	// a tile at scale 1 will contain every 4th point of the tiles at scale 0.
	TILE_SCALE = 4

	// The number of samples per trace in a tile, i.e. the number of git hashes that have data
	// in a single tile.
	TILE_SIZE = 50
)

// Trace represents a single series of measurements. The actual values it
// stores per Commit is defined by implementations of Trace.
type Trace interface {
	// Returns the parameters that describe this trace.
	Params() map[string]string

	// Merge this trace with the given trace. The given trace is expected to come
	// after this trace.
	Merge(Trace) Trace

	DeepCopy() Trace

	// Grow the measurements, filling in with sentinel values either before or
	// after based on FillType.
	Grow(int, FillType)

	// The number of samples in the series.
	Len() int

	// IsMissing returns true if the measurement at index i is a sentinel value,
	// for example, config.MISSING_DATA_SENTINEL.
	IsMissing(i int) bool

	// Trim trims the measurements to just the range from [begin, end).
	//
	// Just like a Go [:] slice this is inclusive of begin and exclusive of end.
	// The length on the Trace will then become end-begin.
	Trim(begin, end int) error

	// Sets the value of the measurement at index.
	//
	// Each specialization will convert []byte to the correct type.
	SetAt(index int, value []byte) error
}

// TraceBuilder builds an empty trace of the correct kind, either a PerfTrace
// or a GoldenTrace.
type TraceBuilder func(n int) Trace

// TraceId helps document when strings should represent TraceIds
type TraceId string

type TraceIdSlice []TraceId

func (b TraceIdSlice) Len() int           { return len(b) }
func (b TraceIdSlice) Less(i, j int) bool { return string(b[i]) < string(b[j]) }
func (b TraceIdSlice) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

// Matches returns true if the given Trace matches the given query.
func Matches(tr Trace, query url.Values) bool {
	for k, values := range query {
		if _, ok := tr.Params()[k]; !ok || !util.In(tr.Params()[k], values) {
			return false
		}
	}
	return true
}

// MatchesWithIgnores returns true if the given Trace matches the given query
// and none of the ignore queries.
func MatchesWithIgnores(tr Trace, query url.Values, ignores ...url.Values) bool {
	if !Matches(tr, query) {
		return false
	}
	for _, i := range ignores {
		if Matches(tr, i) {
			return false
		}
	}
	return true
}

func AsCalculatedID(id string) string {
	if strings.HasPrefix(id, "!") {
		return id
	}
	return "!" + id
}

func IsCalculatedID(id string) bool {
	return strings.HasPrefix(id, "!")
}

func AsFormulaID(id string) string {
	if strings.HasPrefix(id, "@") {
		return id
	}
	return "@" + id
}

func IsFormulaID(id string) bool {
	return strings.HasPrefix(id, "@")
}

func FormulaFromID(id string) string {
	return id[1:]
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

// LastCommitIndex returns the index of the last valid Commit in the given slice of commits.
func LastCommitIndex(commits []*Commit) int {
	for i := len(commits) - 1; i > 0; i-- {
		if commits[i].CommitTime != 0 {
			return i
		}
	}
	return 0
}

// Tile is a config.TILE_SIZE commit slice of data.
//
// The length of the Commits array is the same length as all of the Values
// arrays in all of the Traces.
type Tile struct {
	Traces   map[TraceId]Trace   `json:"traces"`
	ParamSet map[string][]string `json:"param_set"`
	Commits  []*Commit           `json:"commits"`

	// What is the scale of this Tile, i.e. it contains every Nth point, where
	// N=const.TILE_SCALE^Scale.
	Scale     int `json:"scale"`
	TileIndex int `json:"tileIndex"`
}

// NewTile returns an new Tile object.
func NewTile() *Tile {
	t := &Tile{
		Traces:   map[TraceId]Trace{},
		ParamSet: map[string][]string{},
		Commits:  make([]*Commit, TILE_SIZE, TILE_SIZE),
	}
	for i := range t.Commits {
		t.Commits[i] = &Commit{}
	}
	return t
}

// LastCommitIndex returns the index of the last valid Commit.
func (t Tile) LastCommitIndex() int {
	return LastCommitIndex(t.Commits)
}

// Returns the hashes of the first and last commits in the Tile.
func (t Tile) CommitRange() (string, string) {
	return t.Commits[0].Hash, t.Commits[t.LastCommitIndex()].Hash
}

// Makes a copy of the tile where the Traces and Commits are deep copies and
// all the rest of the data is a shallow copy.
func (t Tile) Copy() *Tile {
	ret := &Tile{
		Traces:    map[TraceId]Trace{},
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
		Traces:    map[TraceId]Trace{},
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

// TraceGUI is used in TileGUI.
type TraceGUI struct {
	Data   [][2]float64      `json:"data"`
	Label  string            `json:"label"`
	Params map[string]string `json:"_params"`
}

// TileGUI is the JSON the server serves for tile requests.
type TileGUI struct {
	ParamSet map[string][]string `json:"paramset,omitempty"`
	Commits  []*Commit           `json:"commits,omitempty"`
	Scale    int                 `json:"scale"`
	Tiles    []int               `json:"tiles"`
	Ticks    []interface{}       `json:"ticks"` // The x-axis tick marks.
	Skps     []int               `json:"skps"`  // The x values where SKPs were regenerated.
}

func NewTileGUI(scale int, tileIndex int) *TileGUI {
	return &TileGUI{
		ParamSet: make(map[string][]string, 0),
		Commits:  make([]*Commit, 0),
		Scale:    scale,
		Tiles:    []int{tileIndex},
	}
}

// TileStore is an interface representing the ability to save and restore Tiles.
type TileStore interface {
	Put(scale, index int, tile *Tile) error

	// Get returns the Tile for a given scale and index. Pass in -1 for index to
	// get the last tile for a given scale. Each tile contains its tile index and
	// scale. Get returns (nil, nil) if there is no data in the store yet for that
	// scale and index. The implementation of TileStore can assume that the
	// caller will not modify the tile it returns.
	Get(scale, index int) (*Tile, error)

	// GetModifiable behaves identically to Get, except it always returns a
	// copy that can be modified.
	GetModifiable(scale, index int) (*Tile, error)
}

// Finds the paramSet for the given slice of traces.
func GetParamSet(traces map[TraceId]Trace, paramSet map[string][]string) {
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
		Traces:   make(map[TraceId]Trace),
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
	seen := map[TraceId]bool{}
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
