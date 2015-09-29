package types

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"

	"go.skia.org/infra/go/tiling"
)

func init() {
	// Register *GoldenTrace in gob so that it can be used as a
	// concrete type for Trace when writing and reading Tiles in gobs.
	gob.Register(&GoldenTrace{})
}

const (
	// Primary key field that uniquely identifies a key.
	PRIMARY_KEY_FIELD = "name"

	// Field that contains the corpus identifier.
	CORPUS_FIELD = "source_type"
)

// Label for classifying digests.
type Label int

const (
	// Classifications for observed digests.
	UNTRIAGED Label = iota // == 0
	POSITIVE
	NEGATIVE
)

// String representation for Labels. The order must match order above.
var labelStringRepresentation = []string{
	"untriaged",
	"positive",
	"negative",
}

func (l Label) String() string {
	return labelStringRepresentation[l]
}

var labels = map[string]Label{
	"untriaged": UNTRIAGED,
	"positive":  POSITIVE,
	"negative":  NEGATIVE,
}

func LabelFromString(s string) Label {
	if l, ok := labels[s]; ok {
		return l
	}
	return UNTRIAGED
}

// Stores the digests and their associated labels.
// Note: The name of the test is assumed to be handled by the client of this
// type. Most likely in the keys of a map.
type TestClassification map[string]Label

func (tc *TestClassification) DeepCopy() TestClassification {
	result := make(map[string]Label, len(*tc))
	for k, v := range *tc {
		result[k] = v
	}
	return result
}

const (
	// No digest available.
	MISSING_DIGEST = ""
)

// GoldenTrace represents all the Digests of a single test across a series
// of Commits. GoldenTrace implements the Trace interface.
type GoldenTrace struct {
	Params_ map[string]string
	Values  []string
}

func (g *GoldenTrace) Params() map[string]string {
	return g.Params_
}

func (g *GoldenTrace) Len() int {
	return len(g.Values)
}

func (g *GoldenTrace) IsMissing(i int) bool {
	return g.Values[i] == MISSING_DIGEST
}

func (g *GoldenTrace) DeepCopy() tiling.Trace {
	n := len(g.Values)
	cp := &GoldenTrace{
		Values:  make([]string, n, n),
		Params_: make(map[string]string),
	}
	copy(cp.Values, g.Values)
	for k, v := range g.Params_ {
		cp.Params_[k] = v
	}
	return cp
}

func (g *GoldenTrace) Merge(next tiling.Trace) tiling.Trace {
	nextGold := next.(*GoldenTrace)
	n := len(g.Values) + len(nextGold.Values)
	n1 := len(g.Values)

	merged := NewGoldenTraceN(n)
	merged.Params_ = g.Params_
	for k, v := range nextGold.Params_ {
		merged.Params_[k] = v
	}
	for i, v := range g.Values {
		merged.Values[i] = v
	}
	for i, v := range nextGold.Values {
		merged.Values[n1+i] = v
	}
	return merged
}

func (g *GoldenTrace) Grow(n int, fill tiling.FillType) {
	if n < len(g.Values) {
		panic(fmt.Sprintf("Grow must take a value (%d) larger than the current Trace size: %d", n, len(g.Values)))
	}
	delta := n - len(g.Values)
	newValues := make([]string, n)

	if fill == tiling.FILL_AFTER {
		copy(newValues, g.Values)
		for i := 0; i < delta; i++ {
			newValues[i+len(g.Values)] = MISSING_DIGEST
		}
	} else {
		for i := 0; i < delta; i++ {
			newValues[i] = MISSING_DIGEST
		}
		copy(newValues[delta:], g.Values)
	}
	g.Values = newValues
}

func (g *GoldenTrace) Trim(begin, end int) error {
	if end < begin || end > g.Len() || begin < 0 {
		return fmt.Errorf("Invalid Trim range [%d, %d) of [0, %d]", begin, end, g.Len())
	}
	n := end - begin
	newValues := make([]string, n)

	for i := 0; i < n; i++ {
		newValues[i] = g.Values[i+begin]
	}
	g.Values = newValues
	return nil
}

func (g *GoldenTrace) SetAt(index int, value []byte) error {
	if index < 0 || index > len(g.Values) {
		return fmt.Errorf("Invalid index: %d", index)
	}
	g.Values[index] = string(value)
	return nil
}

// NewGoldenTrace allocates a new Trace set up for the given number of samples.
//
// The Trace Values are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewGoldenTrace() *GoldenTrace {
	return NewGoldenTraceN(tiling.TILE_SIZE)
}

// NewGoldenTraceN allocates a new Trace set up for the given number of samples.
//
// The Trace Values are pre-filled in with the missing data sentinel since not
// all tests will be run on all commits.
func NewGoldenTraceN(n int) *GoldenTrace {
	g := &GoldenTrace{
		Values:  make([]string, n, n),
		Params_: make(map[string]string),
	}
	for i, _ := range g.Values {
		g.Values[i] = MISSING_DIGEST
	}
	return g
}

func GoldenTraceBuilder(n int) tiling.Trace {
	return NewGoldenTraceN(n)
}

// Same as Tile but instead of Traces we preserve the raw JSON. This is a
// utitlity struct that is used to parse a tile where we don't know the
// Trace type upfront.
type TileWithRawTraces struct {
	Traces    map[string]json.RawMessage `json:"traces"`
	ParamSet  map[string][]string        `json:"param_set"`
	Commits   []*tiling.Commit           `json:"commits"`
	Scale     int                        `json:"scale"`
	TileIndex int                        `json:"tileIndex"`
}

// TileFromJson parses a tile that has been serialized to JSON.
// traceExample has to be an instance of the Trace implementation
// that needs to be deserialized.
// Note: Instead of the type switch below we could use reflection
// to be truely generic, but it makes the code harder to read and
// currently we only have two types.
func TileFromJson(r io.Reader, traceExample tiling.Trace) (*tiling.Tile, error) {
	factory := func() tiling.Trace { return NewGoldenTrace() }

	// Decode everything, but the traces.
	dec := json.NewDecoder(r)
	var rawTile TileWithRawTraces
	err := dec.Decode(&rawTile)
	if err != nil {
		return nil, err
	}

	// Parse the traces.
	traces := map[string]tiling.Trace{}
	for k, rawJson := range rawTile.Traces {
		newTrace := factory()
		if err = json.Unmarshal(rawJson, newTrace); err != nil {
			return nil, err
		}
		traces[k] = newTrace.(tiling.Trace)
	}

	return &tiling.Tile{
		Traces:    traces,
		ParamSet:  rawTile.ParamSet,
		Commits:   rawTile.Commits,
		Scale:     rawTile.Scale,
		TileIndex: rawTile.Scale,
	}, nil
}
