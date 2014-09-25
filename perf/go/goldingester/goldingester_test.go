package goldingester

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"skia.googlesource.com/buildbot.git/perf/go/types"
)

func loadDMResults(t *testing.T) *DMResults {
	b, err := ioutil.ReadFile("testdata/dm.json")
	if err != nil {
		t.Fatal("Unable to read file test data.")
	}
	dm := NewDMResults()
	json.Unmarshal(b, dm)
	return dm
}

func TestJSONToDMResults(t *testing.T) {
	dm := loadDMResults(t)

	if got, want := dm.GitHash, "0e9770515cab45decb56a5926d1741b71854fb4c"; got != want {
		t.Errorf("GitHash wrong: Got %v Want %v", got, want)
	}
	if got, want := len(dm.Results), 2; got != want {
		t.Errorf("Results length wrong: Got %v Want %v", got, want)
	}
	if got, want := dm.Results[0].Digest, "445aa63b2200baaba9b37fd5f80c0447"; got != want {
		t.Errorf("Digest wrong: Got %v Want %v", got, want)
	}
	id, params := idAndParams(dm, dm.Results[0])
	if got, want := id, "x86_64:565:Debug:HD7770:ShuttleA:varied_text_clipped_no_lcd:Win8"; got != want {
		t.Errorf("Key generation wrong: Got %v Want %v", got, want)
	}
	if got, want := len(params), 8; got != want {
		t.Errorf("Params wrong size: Got %v Want %v", got, want)
	}
}

func TestParsing(t *testing.T) {
	Init()
	tile := types.NewTile()
	offset := 1
	dm := loadDMResults(t)

	addResultToTile(dm, tile, offset)
	if got, want := len(tile.Traces), 2; got != want {
		t.Errorf("Wrong number of Traces: Got %v Want %v", got, want)
	}
	tr := tile.Traces["x86_64:565:Debug:HD7770:ShuttleA:varied_text_clipped_no_lcd:Win8"].(*types.GoldenTrace)
	if got, want := tr.Values[1], "445aa63b2200baaba9b37fd5f80c0447"; got != want {
		t.Errorf("Digest wrong: Got %v Want %v", got, want)
	}
	if got, want := len(tr.Params()), 8; got != want {
		t.Errorf("Params wrong: Got %v Want %v", got, want)
	}
}
