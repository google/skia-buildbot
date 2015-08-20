// search contains the core functionality for searching for digests across a tile.
package search

import (
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/paramsets"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

// Point is a single point. Used in Trace.
type Point struct {
	X int `json:"x"` // The commit index [0-49].
	Y int `json:"y"`
	S int `json:"s"` // Status of the digest: 0 if the digest matches our search, 1-8 otherwise.
}

// Trace describes a single trace, used in Traces.
type Trace struct {
	Data   []Point           `json:"data"`  // One Point for each test result.
	ID     string            `json:"label"` // The id of the trace. Keep the json as label to be compatible with dots-sk.
	Params map[string]string `json:"params"`
}

// DigestStatus is a digest and its status, used in Traces.
type DigestStatus struct {
	Digest string `json:"digest"`
	Status string `json:"status"`
}

// Traces is info about a group of traces. Used in Digest.
type Traces struct {
	TileSize int                 `json:"tileSize"`
	Traces   []Trace             `json:"traces"`   // The traces where this digest appears.
	Digests  []DigestStatus      `json:"digests"`  // The other digests that appear in Traces.
	ParamSet map[string][]string `json:"paramset"` // The unified params of all the traces where this digest appears.
}

// DiffDigest is information about a digest different from the one in Digest.
type DiffDigest struct {
	Closest  *digesttools.Closest `json:"closest"`
	ParamSet map[string][]string  `json:"paramset"`
}

// Diff is only populated for digests that are untriaged?
// Might still be useful to find diffs to closest pos for a neg, and vice-versa.
// Will also be useful if we ever get a canonical trace or centroid.
type Diff struct {
	Diff float32 `json:"diff"` // The smaller of the Pos and Neg diff.

	// Either may be nil if there's no positive or negative to compare against.
	Pos *DiffDigest `json:"pos"`
	Neg *DiffDigest `json:"neg"`
	//Centroid *DiffDigest

	Blame *blame.BlameDistribution `json:"blame"`
}

// Digest's are returned from Search, one for each match to Query.
type Digest struct {
	Test   string `json:"test"`
	Digest string `json:"digest"`
	Status string `json:"status"`
	Traces Traces `json:"traces"`
	Diff   Diff   `json:"diff"`
}

// CommitRange is a range of commits, starting at the git hash Begin and ending at End, inclusive.
//
// Currently unimplemented in search.
type CommitRange struct {
	Begin string
	End   string
}

// Query is the query that Search understands.
type Query struct {
	BlameGroupID   string // Only applies to Untriaged digests.
	Pos            bool
	Neg            bool
	Unt            bool
	Head           bool
	IncludeIgnores bool
	Query          string
	Issue          string
	CommitRange    CommitRange
	Limit          int // Only return this many items.
}

// intermediate is the intermediate representation of the results coming from Search.
//
// To avoid filtering through the tile more than once we first take a pass
// through the tile and collect all info for the current Query, then we
// transform each intermediate into a Digest.
type intermediate struct {
	Test           string
	Digest         string
	Traces         map[string]tiling.Trace
	SiblingDigests map[string]bool
}

func (i *intermediate) addTrace(id string, tr tiling.Trace, digests []string) {
	i.Traces[id] = tr
	for _, d := range digests {
		i.SiblingDigests[d] = true
	}
}

func newIntermediate(test, digest, id string, tr tiling.Trace, digests []string) *intermediate {
	ret := &intermediate{
		Test:           test,
		Digest:         digest,
		Traces:         map[string]tiling.Trace{},
		SiblingDigests: map[string]bool{},
	}
	ret.addTrace(id, tr, digests)
	return ret
}

// DigestSlice is a utility type for sorting slices of Digest by their max diff.
type DigestSlice []*Digest

func (p DigestSlice) Len() int           { return len(p) }
func (p DigestSlice) Less(i, j int) bool { return p[i].Diff.Diff > p[j].Diff.Diff }
func (p DigestSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Search returns a slice of Digests that match the input query, and the total number of Digests
// that matched the query. It also returns a slice of Commits that were used in the calculations.
func Search(q *Query, storages *storage.Storage, tallies *tally.Tallies, blamer *blame.Blamer, paramset *paramsets.Summary) ([]*Digest, int, []*tiling.Commit, error) {
	tile, err := storages.GetLastTileTrimmed(q.IncludeIgnores)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("Couldn't retrieve tile: %s", err)
	}

	// TODO Use CommitRange to create a trimmed tile.

	parsedQuery, err := url.ParseQuery(q.Query)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("Failed to parse Query in Search: %s", err)
	}
	e, err := storages.ExpectationsStore.Get()
	if err != nil {
		return nil, 0, nil, fmt.Errorf("Couldn't get expectations: %s", err)
	}
	traceTally := tallies.ByTrace()
	lastCommitIndex := tile.LastCommitIndex()

	// Loop over the tile and pull out all the digests that match
	// the query, collecting the matching traces as you go. Build
	// up a set of intermediate's that can then be used to calculate
	// Digest's.

	// map [test:digest] *intermediate
	inter := map[string]*intermediate{}
	for id, tr := range tile.Traces {
		if tiling.Matches(tr, parsedQuery) {
			test := tr.Params()[types.PRIMARY_KEY_FIELD]
			// Get all the digests
			digests := digestsFromTrace(id, tr, q.Head, lastCommitIndex, traceTally)
			for _, digest := range digests {
				cl := e.Classification(test, digest)
				switch {
				case cl == types.NEGATIVE && !q.Neg:
					continue
				case cl == types.POSITIVE && !q.Pos:
					continue
				case cl == types.UNTRIAGED && !q.Unt:
					continue
				}
				// Fix blamer to make this easier.
				if q.BlameGroupID != "" {
					if cl == types.UNTRIAGED {
						b := blamer.GetBlame(test, digest, tile.Commits)
						if q.BlameGroupID != blameGroupID(b, tile.Commits) {
							continue
						}
					} else {
						continue
					}
				}
				key := fmt.Sprintf("%s:%s", test, digest)
				if i, ok := inter[key]; !ok {
					inter[key] = newIntermediate(test, digest, id, tr, digests)
				} else {
					i.addTrace(id, tr, digests)
				}
			}
		}
	}
	// Now loop over all the intermediates and build a Digest for each one.
	ret := make([]*Digest, 0, len(inter))
	for key, i := range inter {
		parts := strings.Split(key, ":")
		ret = append(ret, digestFromIntermediate(parts[0], parts[1], i, e, tile, tallies, blamer, storages.DiffStore, paramset, q.IncludeIgnores))
	}

	sort.Sort(DigestSlice(ret))

	fullLength := len(ret)
	if fullLength > q.Limit {
		ret = ret[0:q.Limit]
	}

	return ret, fullLength, tile.Commits, nil
}

func digestFromIntermediate(test, digest string, inter *intermediate, e *expstorage.Expectations, tile *tiling.Tile, tallies *tally.Tallies, blamer *blame.Blamer, diffStore diff.DiffStore, paramset *paramsets.Summary, includeIgnores bool) *Digest {
	traceTally := tallies.ByTrace()
	ret := &Digest{
		Test:   test,
		Digest: digest,
		Status: e.Classification(test, digest).String(),
		Traces: buildTraces(test, digest, inter, e, tile, traceTally, paramset, includeIgnores),
		Diff:   buildDiff(test, digest, inter, e, tile, tallies.ByTest(), blamer, diffStore, paramset, includeIgnores),
	}
	return ret
}

// buildDiff creates a Diff for the given intermediate.
func buildDiff(test, digest string, inter *intermediate, e *expstorage.Expectations, tile *tiling.Tile, testTally map[string]tally.Tally, blamer *blame.Blamer, diffStore diff.DiffStore, paramset *paramsets.Summary, includeIgnores bool) Diff {
	ret := Diff{
		Diff:  math.MaxFloat32,
		Pos:   nil,
		Neg:   nil,
		Blame: blamer.GetBlame(test, digest, tile.Commits),
	}

	t := testTally[test]
	if t == nil {
		t = tally.Tally{}
	}
	ret.Pos = &DiffDigest{
		Closest: digesttools.ClosestDigest(test, digest, e, t, diffStore, types.POSITIVE),
	}
	ret.Pos.ParamSet = paramset.Get(test, ret.Pos.Closest.Digest, includeIgnores)

	ret.Neg = &DiffDigest{
		Closest: digesttools.ClosestDigest(test, digest, e, t, diffStore, types.NEGATIVE),
	}
	ret.Neg.ParamSet = paramset.Get(test, ret.Neg.Closest.Digest, includeIgnores)

	if pos, neg := ret.Pos.Closest.Diff, ret.Neg.Closest.Diff; pos < neg {
		ret.Diff = pos
	} else {
		ret.Diff = neg
	}

	return ret
}

// buildTraces returns a Trace for the given intermediate.
func buildTraces(test, digest string, inter *intermediate, e *expstorage.Expectations, tile *tiling.Tile, traceTally map[string]tally.Tally, paramset *paramsets.Summary, includeIgnores bool) Traces {
	traceNames := make([]string, 0, len(inter.Traces))
	for id, _ := range inter.Traces {
		traceNames = append(traceNames, id)
	}

	ret := Traces{
		TileSize: len(tile.Commits),
		Traces:   []Trace{},
		Digests:  []DigestStatus{},
		ParamSet: paramset.Get(test, digest, includeIgnores),
	}

	sort.Strings(traceNames)

	last := tile.LastCommitIndex()
	y := 0
	if len(traceNames) > 0 {
		ret.Digests = append(ret.Digests, DigestStatus{
			Digest: digest,
			Status: e.Classification(test, digest).String(),
		})
	}
	for _, id := range traceNames {
		t, ok := traceTally[id]
		if !ok {
			continue
		}
		if count, ok := t[digest]; !ok || count == 0 {
			continue
		}
		trace := inter.Traces[id].(*types.GoldenTrace)
		p := Trace{
			Data:   []Point{},
			ID:     id,
			Params: trace.Params(),
		}
		for i := last; i >= 0; i-- {
			if trace.IsMissing(i) {
				continue
			}
			// s is the status of the digest, it is either 0 for a match, or [1-8] if not.
			s := 0
			if trace.Values[i] != digest {
				if index := digestIndex(trace.Values[i], ret.Digests); index != -1 {
					s = index
				} else {
					if len(ret.Digests) < 9 {
						d := trace.Values[i]
						ret.Digests = append(ret.Digests, DigestStatus{
							Digest: d,
							Status: e.Classification(test, d).String(),
						})
						s = len(ret.Digests) - 1
					} else {
						s = 8
					}
				}
			}
			p.Data = append(p.Data, Point{
				X: i,
				Y: y,
				S: s,
			})
		}
		sort.Sort(PointSlice(p.Data))
		ret.Traces = append(ret.Traces, p)
		y += 1
	}

	return ret
}

// digestIndex returns the index of the digest d in digestInfo, or -1 if not found.
func digestIndex(d string, digestInfo []DigestStatus) int {
	for i, di := range digestInfo {
		if di.Digest == d {
			return i
		}
	}
	return -1
}

// blameGroupID takes a blame distrubution with just indices of commits and
// returns an id for the blame group, which is just a string, the concatenated
// git hashes in commit time order.
func blameGroupID(b *blame.BlameDistribution, commits []*tiling.Commit) string {
	ret := []string{}
	for _, index := range b.Freq {
		ret = append(ret, commits[index].Hash)
	}
	return strings.Join(ret, ":")
}

// digestsFromTrace returns all the digests in the given trace, controlled by
// 'head', and being robust to tallies not having been calculated for the
// trace.
func digestsFromTrace(id string, tr tiling.Trace, head bool, lastCommitIndex int, traceTally map[string]tally.Tally) []string {
	digests := map[string]bool{}
	if head {
		// Find the last non-missing value in the trace.
		for i := lastCommitIndex; i >= 0; i-- {
			if tr.IsMissing(i) {
				continue
			} else {
				digests[tr.(*types.GoldenTrace).Values[i]] = true
				break
			}
		}
	} else {
		// Use the traceTally if available, otherwise just inspect the trace.
		if t, ok := traceTally[id]; ok {
			for k, _ := range t {
				digests[k] = true
			}
		} else {
			for i := lastCommitIndex; i >= 0; i-- {
				if !tr.IsMissing(i) {
					digests[tr.(*types.GoldenTrace).Values[i]] = true
				}
			}
		}
	}

	return util.KeysOfStringSet(digests)
}

// PointSlice is a utility type for sorting Points by their X value.
type PointSlice []Point

func (p PointSlice) Len() int           { return len(p) }
func (p PointSlice) Less(i, j int) bool { return p[i].X < p[j].X }
func (p PointSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
