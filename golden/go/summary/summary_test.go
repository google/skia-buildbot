package summary

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
)

type MockTileStore struct {
	Tile *tiling.Tile
}

func (m MockTileStore) Put(scale, index int, tile *tiling.Tile) error        { return nil }
func (m MockTileStore) GetModifiable(scale, index int) (*tiling.Tile, error) { return nil, nil }
func (m MockTileStore) Get(scale, index int) (*tiling.Tile, error) {
	return m.Tile, nil
}

/**
  Conditions to test.

  Traces
  ------
  id   | config  | test name | corpus(source_type) |  digests
  a      8888      foo         gm                      aaa+, bbb-
  b      565       foo         gm                      ccc?, ddd?
  c      gpu       foo         gm                      eee+
  d      8888      bar         gm                      fff-, ggg?
  e      8888      quux        image                   jjj?

  Expectations
  ------------
  foo  aaa  pos
  foo  bbb  neg
  foo  ccc  unt
  foo  ddd  unt
  foo  eee  pos
  bar  fff  neg

  Ignores
  -------
  config=565

  Note no entry for quux or ggg, meaning untriaged.

  Test the following conditions and make sure you get
  the expected test summaries.

  source_type=gm
    foo - pos(aaa, eee):2  neg(bbb):1
    bar -                  neg(fff):1   unt(ggg):1

  source_type=gm includeIgnores=true
    foo - pos(aaa, eee):2  neg(bbb):1   unt(ccc, ddd):2
    bar -                  neg(fff):1   unt(ggg):1

  source_type=gm includeIgnores=true testName=foo
    foo - pos(aaa, eee):2  neg(bbb):1   unt(ccc, ddd):2

  testname = foo
    foo - pos(aaa, eee):2  neg(bbb):1

  testname = quux
    quux -                              unt(jjj):1

  config=565&config=8888
    foo - pos(aaa):1       neg(bbb):1
    bar -                  neg(fff):1   unt(ggg):1
    quux -                              unt(jjj):1

  config=565&config=8888 head=true
    foo -                  neg(bbb):1
    bar -                               unt(ggg):1
    quux -                              unt(jjj):1

  config=gpu
    foo - pos(eee):1

  config=unknown
    <empty>

*/
func TestCalcSummaries(t *testing.T) {
	tile := &tiling.Tile{
		Traces: map[string]tiling.Trace{
			"a": &types.GoldenTrace{
				Values: []string{"aaa", "bbb"},
				Params_: map[string]string{
					"name":        "foo",
					"config":      "8888",
					"source_type": "gm"},
			},
			"b": &types.GoldenTrace{
				Values: []string{"ccc", "ddd"},
				Params_: map[string]string{
					"name":        "foo",
					"config":      "565",
					"source_type": "gm"},
			},
			"c": &types.GoldenTrace{
				Values: []string{"eee", types.MISSING_DIGEST},
				Params_: map[string]string{
					"name":        "foo",
					"config":      "gpu",
					"source_type": "gm"},
			},
			"d": &types.GoldenTrace{
				Values: []string{"fff", "ggg"},
				Params_: map[string]string{
					"name":        "bar",
					"config":      "8888",
					"source_type": "gm"},
			},
			"e": &types.GoldenTrace{
				Values: []string{"jjj", types.MISSING_DIGEST},
				Params_: map[string]string{
					"name":        "quux",
					"config":      "8888",
					"source_type": "image"},
			},
		},
		Commits: []*tiling.Commit{
			&tiling.Commit{
				CommitTime: 42,
				Hash:       "ffffffffffffffffffffffffffffffffffffffff",
				Author:     "test@test.cz",
			},
			&tiling.Commit{
				CommitTime: 45,
				Hash:       "gggggggggggggggggggggggggggggggggggggggg",
				Author:     "test@test.cz",
			},
		},
		Scale:     0,
		TileIndex: 0,
	}

	eventBus := eventbus.New(nil)
	storages := &storage.Storage{
		DiffStore:         mocks.MockDiffStore{},
		ExpectationsStore: expstorage.NewMemExpectationsStore(eventBus),
		IgnoreStore:       ignore.NewMemIgnoreStore(),
		TileStore:         MockTileStore{Tile: tile},
		NCommits:          50,
		EventBus:          eventBus,
		DigestStore:       &mocks.MockDigestStore{FirstSeen: time.Now().Unix() + 1000, OkValue: true},
	}

	assert.Nil(t, storages.ExpectationsStore.AddChange(map[string]types.TestClassification{
		"foo": map[string]types.Label{
			"aaa": types.POSITIVE,
			"bbb": types.NEGATIVE,
			"ccc": types.UNTRIAGED,
			"ddd": types.UNTRIAGED,
			"eee": types.POSITIVE,
		},
		"bar": map[string]types.Label{
			"fff": types.NEGATIVE,
		},
	}, "foo@example.com"))

	ta, _ := tally.New(storages)
	assert.Nil(t, storages.IgnoreStore.Create(&ignore.IgnoreRule{
		ID:      1,
		Name:    "foo",
		Expires: time.Now().Add(time.Hour),
		Query:   "config=565",
	}))

	blamer, err := blame.New(storages)
	assert.Nil(t, err)

	summaries, err := New(storages, ta, blamer)
	assert.Nil(t, err)

	sum, err := summaries.CalcSummaries(nil, "source_type=gm", false, false)
	if err != nil {
		t.Fatalf("Failed to calc: %s", err)
	}
	assert.Equal(t, 2, len(sum))
	triageCountsCorrect(t, sum, "foo", 2, 1, 0)
	triageCountsCorrect(t, sum, "bar", 0, 1, 1)
	assert.Equal(t, []string{}, sum["foo"].UntHashes)
	assert.Equal(t, []string{"ggg"}, sum["bar"].UntHashes)

	if sum, err = summaries.CalcSummaries(nil, "source_type=gm", true, false); err != nil {
		t.Fatalf("Failed to calc: %s", err)
	}
	assert.Equal(t, 2, len(sum))
	triageCountsCorrect(t, sum, "foo", 2, 1, 2)
	triageCountsCorrect(t, sum, "bar", 0, 1, 1)
	assert.Equal(t, sum["foo"].UntHashes, []string{"ccc", "ddd"})
	assert.Equal(t, sum["bar"].UntHashes, []string{"ggg"})

	if sum, err = summaries.CalcSummaries([]string{"foo"}, "source_type=gm", true, false); err != nil {
		t.Fatalf("Failed to calc: %s", err)
	}
	assert.Equal(t, 1, len(sum))
	triageCountsCorrect(t, sum, "foo", 2, 1, 2)
	assert.Equal(t, sum["foo"].UntHashes, []string{"ccc", "ddd"})

	if sum, err = summaries.CalcSummaries([]string{"foo"}, "", false, false); err != nil {
		t.Fatalf("Failed to calc: %s", err)
	}
	assert.Equal(t, 1, len(sum))
	triageCountsCorrect(t, sum, "foo", 2, 1, 0)
	assert.Equal(t, sum["foo"].UntHashes, []string{})

	if sum, err = summaries.CalcSummaries(nil, "config=8888&config=565", false, false); err != nil {
		t.Fatalf("Failed to calc: %s", err)
	}
	assert.Equal(t, 3, len(sum))
	triageCountsCorrect(t, sum, "foo", 1, 1, 0)
	triageCountsCorrect(t, sum, "bar", 0, 1, 1)
	triageCountsCorrect(t, sum, "quux", 0, 0, 1)
	assert.Equal(t, sum["foo"].UntHashes, []string{})
	assert.Equal(t, sum["bar"].UntHashes, []string{"ggg"})
	assert.Equal(t, sum["quux"].UntHashes, []string{"jjj"})

	if sum, err = summaries.CalcSummaries(nil, "config=8888&config=565", false, true); err != nil {
		t.Fatalf("Failed to calc: %s", err)
	}
	assert.Equal(t, 3, len(sum))
	triageCountsCorrect(t, sum, "foo", 0, 1, 0)
	triageCountsCorrect(t, sum, "bar", 0, 0, 1)
	triageCountsCorrect(t, sum, "quux", 0, 0, 1)
	assert.Equal(t, sum["foo"].UntHashes, []string{})
	assert.Equal(t, sum["bar"].UntHashes, []string{"ggg"})
	assert.Equal(t, sum["quux"].UntHashes, []string{"jjj"})

	if sum, err = summaries.CalcSummaries(nil, "config=gpu", false, false); err != nil {
		t.Fatalf("Failed to calc: %s", err)
	}
	assert.Equal(t, 1, len(sum))
	triageCountsCorrect(t, sum, "foo", 1, 0, 0)
	assert.Equal(t, sum["foo"].UntHashes, []string{})

	if sum, err = summaries.CalcSummaries(nil, "config=unknown", false, false); err != nil {
		t.Fatalf("Failed to calc: %s", err)
	}
	assert.Equal(t, 0, len(sum))
}

func triageCountsCorrect(t *testing.T, sum map[string]*Summary, name string, pos, neg, unt int) {
	s := sum[name]
	if got, want := s.Pos, pos; got != want {
		t.Errorf("Positive count %s: Got %v Want %v", name, got, want)
	}
	if got, want := s.Neg, neg; got != want {
		t.Errorf("Negative count %s: Got %v Want %v", name, got, want)
	}
	if got, want := s.Untriaged, unt; got != want {
		t.Errorf("Untriaged count %s: Got %v Want %v", name, got, want)
	}
}
