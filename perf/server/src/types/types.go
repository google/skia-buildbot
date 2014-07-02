package types

import (
	"time"
)

import (
	"config"
)

// Trace represents all the values of a single measurement over time.
type Trace struct {
	Key    string            `json:"key"`
	Values []float64         `json:"values"`
	Params map[string]string `json:"params"`
	Trybot bool              `json:"trybot"`
}

// NewTrace allocates a new Trace set up for the given number of samples.
//
// The Trace Values are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewTrace(numSamples int) *Trace {
	t := &Trace{
		Values: make([]float64, numSamples, numSamples),
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

// Choices is a list of possible values for a param. See Tile.
type Choices []string

// Tile is a 32 commit slice of data.
//
// The length of the Commits array is the same length as all of the Values
// arrays in all of the Traces.
type Tile struct {
	Traces   []*Trace           `json:"traces"`
	ParamSet map[string]Choices `json:"param_set"`
	Commits  []*Commit          `json:"commits"`

	// What is the scale of this Tile, i.e. it contains every Nth point, where
	// N=const.TILE_SCALE^Scale.
	Scale     int `json:"scale"`
	TileIndex int `json:"tileIndex"`
}

// NewTile returns an new Tile object ready to be filled with data via populate().
func NewTile() *Tile {
	return &Tile{
		Traces:   make([]*Trace, 0),
		ParamSet: make(map[string]Choices),
		Commits:  make([]*Commit, 0),
	}
}

// TileStore is an interface representing the ability to save and restore Tiles.
type TileStore interface {
	Put(scale, index int, tile *Tile) error

	// Get returns the Tile for a given scale and index. Pass in -1 for index to
	// get the last tile for a given scale. Each tile contains its tile index and
	// scale. Get returns (nil, nil) if you pass in -1 and there is no data in
	// the store yet.
	Get(scale, index int) (*Tile, error)
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
