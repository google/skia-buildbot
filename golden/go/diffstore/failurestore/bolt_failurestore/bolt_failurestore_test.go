package bolt_failurestore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

func TestAddGet(t *testing.T) {
	unittest.MediumTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()

	fs, err := New(w)
	require.NoError(t, err)

	unavailable, err := fs.UnavailableDigests()
	require.NoError(t, err)
	require.Empty(t, unavailable)

	err = fs.AddDigestFailure(&failureOne)
	require.NoError(t, err)
	err = fs.AddDigestFailure(&failureTwo)
	require.NoError(t, err)
	err = fs.AddDigestFailure(&failureThree)
	require.NoError(t, err)

	unavailable, err = fs.UnavailableDigests()
	require.NoError(t, err)
	require.Equal(t, map[types.Digest]*diff.DigestFailure{
		digestOne: &failureThree,
		digestTwo: &failureTwo,
	}, unavailable)
}

func TestAddIfNew(t *testing.T) {
	unittest.MediumTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()

	fs, err := New(w)
	require.NoError(t, err)

	err = fs.AddDigestFailureIfNew(&failureOne)
	require.NoError(t, err)
	err = fs.AddDigestFailureIfNew(&failureTwo)
	require.NoError(t, err)
	err = fs.AddDigestFailureIfNew(&failureThree)
	require.NoError(t, err)

	unavailable, err := fs.UnavailableDigests()
	require.NoError(t, err)
	require.Equal(t, map[types.Digest]*diff.DigestFailure{
		digestOne: &failureOne,
		digestTwo: &failureTwo,
	}, unavailable)
}

func TestPurge(t *testing.T) {
	unittest.MediumTest(t)

	w, cleanup := testutils.TempDir(t)
	defer cleanup()

	fs, err := New(w)
	require.NoError(t, err)

	err = fs.AddDigestFailure(&failureOne)
	require.NoError(t, err)
	err = fs.AddDigestFailure(&failureTwo)
	require.NoError(t, err)
	err = fs.PurgeDigestFailures(types.DigestSlice{digestOne})
	require.NoError(t, err)

	unavailable, err := fs.UnavailableDigests()
	require.NoError(t, err)
	require.Equal(t, map[types.Digest]*diff.DigestFailure{
		digestTwo: &failureTwo,
	}, unavailable)
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
