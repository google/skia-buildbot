package gtile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testMap = map[string]string{
		"key1": "val1",
		"key2": "val2",
	}
	testSlice = []string{
		"digest-1",
		"digest-2",
		"digest-3",
		"digest-4",
	}
)

func TestStrMap(t *testing.T) {
	strMapping := StrMap{}
	strMapping.init(100)

	intMap := strMapping.intMap(testMap)
	assert.Equal(t, len(testMap), len(intMap))
	strMap := strMapping.strMap(intMap)
	assert.Equal(t, testMap, strMap)
}
