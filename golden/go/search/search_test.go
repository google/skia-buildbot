package search

import (
	"os"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/golden/go/serialize"
)

func loadSample(t assert.TestingT, fileName string) *serialize.Sample {
	file, err := os.Open(fileName)
	assert.NoError(t, err)

	sample, err := serialize.DeserializeSample(file)
	assert.NoError(t, err)

	return sample
}
