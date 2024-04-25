package ingest

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/ingest/format"
	"go.skia.org/infra/perf/go/perfresults"
)

func loadTestdata(t assert.TestingT, filename string) *perfresults.PerfResults {
	var r io.Reader
	r, err := os.Open("../testdata/" + filename)
	assert.NoError(t, err)

	pr, err := perfresults.NewResults(r)
	assert.NoError(t, err)
	return pr
}

func loadTestdataUnmarshal(filename string) *perfresults.PerfResults {
	d, _ := os.ReadFile("../testdata/" + filename)

	pr := &perfresults.PerfResults{}
	_ = pr.UnmarshalJSON(d)
	return pr
}

func Test_EncodeFormat_PerfResults_ReturnsValidJson(t *testing.T) {
	validate := func(filename string, links map[string]string) {
		pr := loadTestdata(t, filename)
		f := ConvertPerfResultsFormat(pr, "CP:1", nil, links)

		b, err := json.Marshal(f)
		assert.NoError(t, err)
		msg, err := format.Validate(bytes.NewReader(b))
		assert.NoErrorf(t, err, "violations: %v", msg)
	}

	validate("full.json", map[string]string{})
	validate("full.json", map[string]string{
		"trace_link": "https://www.this.is.com/links/traceXXX",
	})
	validate("full.json", nil)
	validate("empty.json", nil)
	validate("merged_diff.json", nil)
	validate("valid_metadata.json", nil)
	validate("valid_histograms.json", nil)
}

func BenchmarkLoadPerfResultsJSON(b *testing.B) {
	b.Run("unmarshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = loadTestdataUnmarshal("full.json")
		}
	})
	b.Run("decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = loadTestdata(b, "full.json")
		}
	})
}
