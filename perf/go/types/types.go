package types

import "time"

import (
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/util"
)

// Trace represents all the values of a single measurement over time.
type Trace struct {
	Values []float64         `json:"values"`
	Trybot bool              `json:"trybot"`
	Params map[string]string `json:"params"`
}

// NewTrace allocates a new Trace set up for the given number of samples.
//
// The Trace Values are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewTrace() *Trace {
	return newTraceN(config.TILE_SIZE)
}

// newTraceN allocates a new Trace set up for the given number of samples.
//
// The Trace Values are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func newTraceN(n int) *Trace {
	t := &Trace{
		Values: make([]float64, n, n),
		Params: make(map[string]string),
		Trybot: false,
	}
	for i, _ := range t.Values {
		t.Values[i] = config.MISSING_DATA_SENTINEL
	}
	return t
}

// Annotations for commits.
//
// Will map to the table of annotation notes in MySQL. See DESIGN.md
// for the MySQL schema.
type Annotation struct {
	ID     int    `json:"id"     db:"id"`
	Notes  string `json:"notes"  db:"notes"`
	Author string `json:"author" db:"author"`
	Type   int    `json:"type"   db:"type"`
}

// Commit is information about each Git commit.
type Commit struct {
	CommitTime    int64     `json:"commit_time" bq:"timestamp" db:"ts"`
	Hash          string    `json:"hash"        bq:"gitHash"   db:"githash"`
	GitNumber     int64     `json:"git_number"  bq:"gitNumber" db:"gitnumber"`
	Author        string    `json:"author"                     db:"author"`
	CommitMessage string    `json:"commit_msg"                 db:"message"`
	TailCommits   []*Commit `json:"tail_commits,omitempty"`
}

func NewCommit() *Commit {
	return &Commit{
		TailCommits: []*Commit{},
	}
}

// Tile is a config.TILE_SIZE commit slice of data.
//
// The length of the Commits array is the same length as all of the Values
// arrays in all of the Traces.
type Tile struct {
	Traces   map[string]*Trace   `json:"traces"`
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
		Traces:   make(map[string]*Trace),
		ParamSet: make(map[string][]string),
		Commits:  make([]*Commit, config.TILE_SIZE, config.TILE_SIZE),
	}
	for i := range t.Commits {
		t.Commits[i] = NewCommit()
	}
	return t
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
	// scale. Get returns (nil, nil) if you pass in -1 and there is no data in
	// the store yet. The implementation of TileStore can assume that
	// the caller will not modify the tile it returns.
	Get(scale, index int) (*Tile, error)

	// GetModifiable behaves identically to Get, except it always returns a
	// copy that can be modified.
	GetModifiable(scale, index int) (*Tile, error)
}

// DateIter allows for easily iterating backwards, one day at a time, until
// reaching the BEGINNING_OF_TIME.
type DateIter struct {
	day       time.Time
	firstLoop bool
}

func NewDateIter() *DateIter {
	return &DateIter{
		day:       time.Now(),
		firstLoop: true,
	}
}

// Next is the iterator step function to be used in a for loop.
func (i *DateIter) Next() bool {
	if i.firstLoop {
		i.firstLoop = false
		return true
	}
	i.day = i.day.Add(-24 * time.Hour)
	return i.Date() != config.BEGINNING_OF_TIME.BqTableSuffix()
}

// Date returns the day formatted as we use them on BigQuery table name suffixes.
func (i *DateIter) Date() string {
	return i.day.Format("20060102")
}

// Merge the two Tiles, presuming tile1 comes before tile2.
func Merge(tile1, tile2 *Tile) *Tile {
	n := len(tile1.Commits) + len(tile2.Commits)
	n1 := len(tile1.Commits)
	t := &Tile{
		Traces:   make(map[string]*Trace),
		ParamSet: make(map[string][]string),
		Commits:  make([]*Commit, n, n),
	}
	for i := range t.Commits {
		t.Commits[i] = NewCommit()
	}

	// Merge the Commits.
	for i, c := range tile1.Commits {
		t.Commits[i] = c
	}
	for i, c := range tile2.Commits {
		t.Commits[n1+i] = c
	}

	// Merge the Traces.
	seen := map[string]bool{}
	for key, trace := range tile1.Traces {
		seen[key] = true
		mergedTrace := newTraceN(n)
		mergedTrace.Params = trace.Params
		for i, v := range trace.Values {
			mergedTrace.Values[i] = v
		}
		if trace2, ok := tile2.Traces[key]; ok {
			for i, v := range trace2.Values {
				mergedTrace.Values[n1+i] = v
			}
		}
		t.Traces[key] = mergedTrace
	}
	// Now add in the traces that are only in tile2.
	for key, trace := range tile2.Traces {
		if _, ok := seen[key]; ok {
			continue
		}
		mergedTrace := newTraceN(n)
		mergedTrace.Params = trace.Params
		for i, v := range trace.Values {
			mergedTrace.Values[n1+i] = v
		}
		t.Traces[key] = mergedTrace
	}

	// Recreate the ParamSet.
	for _, trace := range t.Traces {
		for k, v := range trace.Params {
			if _, ok := t.ParamSet[k]; !ok {
				t.ParamSet[k] = []string{v}
			} else if !util.In(v, t.ParamSet[k]) {
				t.ParamSet[k] = append(t.ParamSet[k], v)
			}
		}
	}

	t.Scale = tile1.Scale
	t.TileIndex = tile1.TileIndex

	return t
}
