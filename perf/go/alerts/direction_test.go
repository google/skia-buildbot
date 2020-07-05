package alerts

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

type testDirStruct struct {
	Direction Direction
}

func TestDirectionJSON(t *testing.T) {
	unittest.SmallTest(t)
	ts := testDirStruct{
		Direction: UP,
	}
	b, err := json.Marshal(ts)
	assert.NoError(t, err)
	assert.Equal(t, "{\"Direction\":\"UP\"}", string(b))

	target := &testDirStruct{}
	err = json.Unmarshal(b, target)
	assert.NoError(t, err)
	assert.Equal(t, UP, target.Direction)
}
