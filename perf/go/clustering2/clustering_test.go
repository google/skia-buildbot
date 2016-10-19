package clustering2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/perf/go/ctrace2"
	"go.skia.org/infra/perf/go/kmeans"
)

func TestParamSummaries(t *testing.T) {
	obs := []kmeans.Clusterable{
		ctrace2.NewFullTrace(",arch=x86,config=8888,", []float32{1, 2}, 0.001),
		ctrace2.NewFullTrace(",arch=x86,config=565,scale=1,", []float32{2, 3}, 0.001),
		ctrace2.NewFullTrace(",arch=x86,config=565,scale=1.1,", []float32{2, 3}, 0.001),
		ctrace2.NewFullTrace(",arch=x86,config=gpu,", []float32{3, 4}, 0.001),
	}
	expected := map[string][]ValueWeight{
		"arch": []ValueWeight{
			{"x86", 26},
		},
		"config": []ValueWeight{
			{"565", 19},
			{"8888", 15},
			{"gpu", 15},
		},
		"scale": []ValueWeight{
			{"1", 15},
			{"1.1", 15},
		},
	}
	assert.Equal(t, expected, getParamSummaries(obs))

	obs = []kmeans.Clusterable{}
	expected = map[string][]ValueWeight{}
	assert.Equal(t, expected, getParamSummaries(obs))
}
