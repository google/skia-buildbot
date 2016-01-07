package valueweight

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/types"
)

func TestFromParams(t *testing.T) {
	traceparams := []map[string]string{
		map[string]string{
			"config": "8888",
			"cpu":    "intel",
		},
		map[string]string{
			"config": "565",
			"cpu":    "intel",
		},
		map[string]string{
			"config": "gpu",
			"cpu":    "arm",
		},
	}
	wordcloud := FromParams(traceparams)
	expected := [][]types.ValueWeight{
		[]types.ValueWeight{
			types.ValueWeight{Value: "intel", Weight: 21},
			types.ValueWeight{Value: "arm", Weight: 16},
		},
		[]types.ValueWeight{
			types.ValueWeight{Value: "8888", Weight: 16},
			types.ValueWeight{Value: "565", Weight: 16},
			types.ValueWeight{Value: "gpu", Weight: 16},
		},
	}

	assert.Equal(t, expected, wordcloud)
}
