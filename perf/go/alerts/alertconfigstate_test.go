package alerts

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

type testStruct struct {
	State ConfigState
}

func TestJSON(t *testing.T) {
	unittest.SmallTest(t)
	ts := testStruct{
		State: DELETED,
	}
	b, err := json.Marshal(ts)
	assert.NoError(t, err)
	assert.Equal(t, "{\"State\":\"DELETED\"}", string(b))

	target := &testStruct{}
	err = json.Unmarshal(b, target)
	assert.NoError(t, err)
	assert.Equal(t, DELETED, target.State)

	target = &testStruct{}
	err = json.Unmarshal([]byte("{\"State\":\"NOT A VALID VALUE\"}"), target)
	assert.NoError(t, err)
	assert.Equal(t, ACTIVE, target.State)
}
