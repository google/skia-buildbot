package bt_tracestore

import (
	"math/rand"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

func TestDigestMap(t *testing.T) {
	unittest.SmallTest(t)

	digestMap := NewDigestMap(1000)
	assert.Equal(t, 1, digestMap.Len())
	id, err := digestMap.ID("")
	assert.NoError(t, err)
	assert.Equal(t, id, DigestID(0))
	digests, err := digestMap.DecodeIDs([]DigestID{0})
	assert.NoError(t, err)
	assert.Equal(t, digests, []types.Digest{""})
	assert.Error(t, digestMap.Add(map[types.Digest]DigestID{"": 5}))
	assert.Error(t, digestMap.Add(map[types.Digest]DigestID{"somedigest": 0}))

	expDigestSet := map[types.Digest]bool{}
	expDigests := []types.Digest{""}
	expIDs := []DigestID{0}

	n := 1000
	idCounter := DigestID(1)
	expMapping := map[types.Digest]DigestID{}
	for i := 0; i < n; i++ {
		d := randomDigest()
		expMapping[d] = idCounter
		expDigestSet[d] = true
		expDigests = append(expDigests, d)
		expIDs = append(expIDs, idCounter)
		idCounter++
	}
	assert.NoError(t, digestMap.Add(expMapping))
	assert.Error(t, digestMap.Add(map[types.Digest]DigestID{randomDigest(): idCounter - 1}))
	assert.Equal(t, 0, len(digestMap.Delta(expDigestSet)))

	for digest, expID := range expMapping {
		id, err := digestMap.ID(digest)
		assert.NoError(t, err)
		assert.Equal(t, expID, id)
	}

	actualDigests, err := digestMap.DecodeIDs(expIDs)
	assert.NoError(t, err)
	assert.Equal(t, expDigests, actualDigests)
}

const hexLetters = "0123456789abcdef"
const md5Length = 32

func randomDigest() types.Digest {
	ret := make([]byte, md5Length)
	for i := 0; i < md5Length; i++ {
		ret[i] = hexLetters[rand.Intn(len(hexLetters))]
	}
	return types.Digest(ret)
}
