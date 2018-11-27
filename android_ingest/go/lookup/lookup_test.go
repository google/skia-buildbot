package lookup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

func TestParseLog(t *testing.T) {
	testutils.SmallTest(t)
	c := &Cache{
		hashes: map[int64]string{},
	}
	log := `6dab50c23b3927daf7487b4a6f105fc74aff5fa7 https://android-ingest.skia.org/r/3553310
3133350e05eb07629d681c3bb61a91a51e2ff2ef https://android-ingest.skia.org/r/3553227
eceadc0434451cfdce5dc6814cd48ef0f36b1dc2 https://android-ingest.skia.org/r/3553052?branch=foo
716b074f2a057324148d1af51fedd30c603da538 https://android-ingest.skia.org/r/3553049
`
	err := c.parseLog(log)
	assert.NoError(t, err)
	assert.Len(t, c.hashes, 4)
	assert.Equal(t, "eceadc0434451cfdce5dc6814cd48ef0f36b1dc2", c.hashes[3553052])

	hash, err := c.Lookup(3553052)
	assert.NoError(t, err)
	assert.Equal(t, "eceadc0434451cfdce5dc6814cd48ef0f36b1dc2", hash)

	hash, err = c.Lookup(1234)
	assert.Error(t, err)
	assert.Equal(t, "", hash)

	c.Add(1234, "aaaabbbbcccc")

	hash, err = c.Lookup(1234)
	assert.NoError(t, err)
	assert.Equal(t, "aaaabbbbcccc", hash)
}
