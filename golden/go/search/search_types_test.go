package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

var (
	TEST_1    = types.TestName("test-1")
	DIGEST_01 = types.Digest("abcefgh")
	PARAMS_01 = map[string][]string{
		"param-01": {"val-01"},
		"param-02": {"val-02"},
	}
)

func TestIntermediate(t *testing.T) {
	testutils.SmallTest(t)

	srMap := srInterMap{}
	srMap.add(TEST_1, DIGEST_01, "", nil, PARAMS_01)
	assert.Equal(t, srInterMap{TEST_1: map[types.Digest]*srIntermediate{
		DIGEST_01: {
			test:   TEST_1,
			digest: DIGEST_01,
			params: PARAMS_01,
			traces: map[tiling.TraceId]*types.GoldenTrace{},
		},
	}}, srMap)
}
