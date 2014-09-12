package importer

import (
	"io/ioutil"
	"testing"
)

import (
	"github.com/stretchr/testify/assert"
)

// the input file
const (
	TEST_INPUT_FNAME = "testdata/expected-results.json"
	DIGEST_ID        = "bitmap-64bitMD5"
)

// Constants, but need to pass a pointer to them.
var (
	TRUE  = true
	FALSE = false
)

func TestReadLegacyResults(t *testing.T) {
	jsonData, err := ioutil.ReadFile(TEST_INPUT_FNAME)
	if err != nil {
		t.Fatal("Unable to read file " + TEST_INPUT_FNAME)
	}

	data, err := DecodeLegacyResults(jsonData)
	if err != nil {
		t.Error("Unable to decode input data. Msg: " + err.Error())
	}

	// assert the size of the expected results
	assert.Equal(t, 4, len(data.ActualResults))
	assert.Equal(t, 3, len(data.ExpectedResults))

	checkEntry(t, data.ExpectedResults["3x3bitmaprect_565.png"], 16998423976396106083, []int{1578}, &FALSE, nil)
	checkEntry(t, data.ExpectedResults["alphagradients_gpu.png"], 4424928001525270278, nil, nil, &FALSE)
	checkEntry(t, data.ExpectedResults["bigtext_gpu.png"], 11021706081645443796, nil, &TRUE, &FALSE)
}

func checkEntry(t *testing.T, e *ExpectedResult, dVal uint64, bugs []int, rBH *bool, igF *bool) {
	assert.Equal(t, 1, len(e.AllowedDigests))
	assert.Equal(t, *e.AllowedDigests[0], AllowedDigest{DIGEST_ID, dVal})
	assert.Equal(t, e.Bugs, bugs)
	checkBool(t, rBH, e.ReviewedByHuman)
	checkBool(t, igF, e.IgnoreFailure)
}

func checkBool(t *testing.T, exp *bool, act *bool) {
	if exp == nil {
		assert.Nil(t, act)
	} else {
		assert.Equal(t, *exp, *act)
	}
}
