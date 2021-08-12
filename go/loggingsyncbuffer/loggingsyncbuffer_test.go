package loggingsyncbuffer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestSyncWriter_WriteString_StringIsReturned(t *testing.T) {
	unittest.SmallTest(t)
	sw := New()
	const s = "Hello World!"
	_, err := sw.Write([]byte(s))
	assert.NoError(t, err)
	assert.Equal(t, s, sw.String())
}

func TestSyncWriter_Sync_ReturnsNil(t *testing.T) {
	unittest.SmallTest(t)
	sw := New()
	assert.Nil(t, sw.Sync())
}
