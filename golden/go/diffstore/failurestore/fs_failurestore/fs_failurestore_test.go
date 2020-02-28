package fs_failurestore

import (
	"context"
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
	c, cleanup := firestore.NewClientForTesting(context.Background(), t)
	defer cleanup()

	fs := New(c)

	ctx := context.Background()
	unavailableDigests, err := fs.UnavailableDigests(ctx)
	assert.NoError(t, err)
	assert.Empty(t, unavailableDigests)

	err = fs.AddDigestFailure(ctx, &failureOne)
	assert.NoError(t, err)
	err = fs.AddDigestFailure(ctx, &failureTwo)
	assert.NoError(t, err)
	err = fs.AddDigestFailure(ctx, &failureThree)
	assert.NoError(t, err)

	unavailableDigests, err = fs.UnavailableDigests(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[types.Digest]*diff.DigestFailure{
		digestOne: &failureThree,
		digestTwo: &failureTwo,
	}, unavailableDigests)
}

func TestPurge(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(context.Background(), t)
	defer cleanup()

	fs := New(c)

	ctx := context.Background()
	err := fs.AddDigestFailure(ctx, &failureOne)
	assert.NoError(t, err)
	err = fs.AddDigestFailure(ctx, &failureTwo)
	assert.NoError(t, err)
	err = fs.PurgeDigestFailures(ctx, types.DigestSlice{digestOne})
	assert.NoError(t, err)

	unavailableDigests, err := fs.UnavailableDigests(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[types.Digest]*diff.DigestFailure{
		digestTwo: &failureTwo,
	}, unavailableDigests)
}

const (
	digestOne = types.Digest("ab592bfb76536d833e16028bf9525508")
	digestTwo = types.Digest("9a58d5801670ef194eba7451b08621ac")
)

var (
	now = time.Date(2019, time.June, 3, 4, 5, 16, 0, time.UTC)

	failureOne = diff.DigestFailure{
		Digest: digestOne,
		Reason: "http_error",
		TS:     now.Unix() * 1000,
	}
	failureTwo = diff.DigestFailure{
		Digest: digestTwo,
		Reason: "http_error",
		TS:     now.Unix()*1000 + 2345,
	}
	failureThree = diff.DigestFailure{
		Digest: digestOne,
		Reason: "http_error",
		TS:     now.Unix()*1000 + 6789,
	}
)
