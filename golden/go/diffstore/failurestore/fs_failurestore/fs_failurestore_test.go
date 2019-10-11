package fs_failurestore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

func TestAddGet(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	fs := New(c)

	assert.Empty(t, fs.UnavailableDigests())

	err := fs.AddDigestFailure(&failureOne)
	assert.NoError(t, err)
	err = fs.AddDigestFailure(&failureTwo)
	assert.NoError(t, err)
	err = fs.AddDigestFailure(&failureThree)
	assert.NoError(t, err)

	assert.Equal(t, map[types.Digest]*diff.DigestFailure{
		digestOne: &failureThree,
		digestTwo: &failureTwo,
	}, fs.UnavailableDigests())
}

func TestAddIfNew(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	fs := New(c)

	err := fs.AddDigestFailureIfNew(&failureOne)
	assert.NoError(t, err)
	err = fs.AddDigestFailureIfNew(&failureTwo)
	assert.NoError(t, err)
	err = fs.AddDigestFailureIfNew(&failureThree)
	assert.NoError(t, err)

	assert.Equal(t, map[types.Digest]*diff.DigestFailure{
		digestOne: &failureOne,
		digestTwo: &failureTwo,
	}, fs.UnavailableDigests())
}

func TestPurge(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	fs := New(c)

	err := fs.AddDigestFailure(&failureOne)
	assert.NoError(t, err)
	err = fs.AddDigestFailure(&failureTwo)
	assert.NoError(t, err)
	err = fs.PurgeDigestFailures(types.DigestSlice{digestOne})
	assert.NoError(t, err)

	assert.Equal(t, map[types.Digest]*diff.DigestFailure{
		digestTwo: &failureTwo,
	}, fs.UnavailableDigests())
}

const (
	digestOne = types.Digest("ab592bfb76536d833e16028bf9525508")
	digestTwo = types.Digest("9a58d5801670ef194eba7451b08621ac")
)

var (
	now = time.Date(2019, time.June, 3, 4, 5, 16, 0, time.UTC)

	failureOne = diff.DigestFailure{
		Digest: digestOne,
		Reason: "404",
		TS:     now.Unix() * 1000,
	}
	failureTwo = diff.DigestFailure{
		Digest: digestTwo,
		Reason: "417",
		TS:     now.Unix()*1000 + 2345,
	}
	failureThree = diff.DigestFailure{
		Digest: digestOne,
		Reason: "500",
		TS:     now.Unix()*1000 + 6789,
	}
)
