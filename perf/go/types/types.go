package types

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

// LastCommitIndex returns the index of the last valid Commit.
func (t Tile) LastCommitIndex() int {
	for i := len(t.Commits) - 1; i > 0; i-- {
		if t.Commits[i].CommitTime != 0 {
			return i
		}
	}
	return 0
}

// Returns the hashes of the first and last commits in the Tile.
func (t Tile) CommitRange() (string, string) {
	return t.Commits[0].Hash, t.Commits[t.LastCommitIndex()].Hash
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
	// scale. Get returns (nil, nil) if you pass in -1 and there is no data in
	// the store yet. The implementation of TileStore can assume that
	// the caller will not modify the tile it returns.
	Get(scale, index int) (*Tile, error)

	// GetModifiable behaves identically to Get, except it always returns a
	// copy that can be modified.
	GetModifiable(scale, index int) (*Tile, error)
}

// ValueWeight is a weight proportional to the number of times the parameter
// Value appears in a cluster. Used in ClusterSummary.
type ValueWeight struct {
	Value  string
	Weight int
}

// StepFit stores information on the best Step Function fit on a trace.
//
// Used in ClusterSummary.
type StepFit struct {
	// LeastSquares is the Least Squares error for a step function curve fit to the trace.
	LeastSquares float64

	// TurningPoint is the index where the Step Function changes value.
	TurningPoint int

	// StepSize is the size of the step in the step function. Negative values
	// indicate a step up, i.e. they look like a performance regression in the
	// trace, as opposed to positive values which look like performance
	// improvements.
	StepSize float64

	// The "Regression" value is calculated as Step Size / Least Squares Error.
	//
	// The better the fit the larger the number returned, because LSE
	// gets smaller with a better fit. The higher the Step Size the
	// larger the number returned.
	Regression float64

	// Status of the cluster.
	//
	// Values can be "High", "Low", and "Uninteresting"
	Status string
}

// ClusterSummary is a summary of a single cluster of traces.
type ClusterSummary struct {
	// Traces contains at most config.MAX_SAMPLE_TRACES_PER_CLUSTER sample
	// traces, the first is the centroid.
	Traces [][][]float64

	// Keys of all the members of the Cluster.
	Keys []string

	// ParamSummaries is a summary of all the parameters in the cluster.
	ParamSummaries [][]ValueWeight

	// StepFit is info on the fit of the centroid to a step function.
	StepFit *StepFit

	// Hash is the Git hash at the step point.
	Hash string

	// Timestamp is when this hash was committed.
	Timestamp int64

	// Status is the status, "New", "Ingore" or "Bug".
	Status string

	// A note about the Status.
	Message string

	// ID is the identifier for this summary in the datastore.
	ID int64
}

func NewClusterSummary(numKeys, numTraces int) *ClusterSummary {
	return &ClusterSummary{
		Keys:           make([]string, numKeys),
		Traces:         make([][][]float64, numTraces),
		ParamSummaries: [][]ValueWeight{},
		StepFit:        &StepFit{},
		Hash:           "",
		Timestamp:      0,
		Status:         "New",
		Message:        "",
		ID:             -1,
	}
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
