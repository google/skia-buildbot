package data_kitchen_sink_test

import (
	"image"
	"image/png"
	"io"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"

	. "go.skia.org/infra/golden/go/testutils/data_kitchen_sink"
)

func TestMakeTraces_CorrectIDs(t *testing.T) {
	traces := MakeTraces()
	uniqueIds := map[tiling.TraceID]bool{}
	for _, tp := range traces {
		assert.Equal(t, tiling.TraceIDFromParams(tp.Trace.Keys()), tp.ID)
		assert.NotContains(t, uniqueIds, tp.ID, "traces should be unique - %s was not", tp.ID)
		uniqueIds[tp.ID] = true
	}
}

func TestMakeTraces_CorrectNumberOfDigests(t *testing.T) {
	traces := MakeTraces()
	for _, tp := range traces {
		assert.Len(t, tp.Trace.Digests, NumCommits)
		assert.Equal(t, NumCommits, tp.Trace.Len())
	}
}

func TestMakeDiffsForCorpusNameGrouping_HasCorrectData(t *testing.T) {
	groups := map[string][]types.Digest{
		`{"corpus":"round","name":"circle"}`:     {DigestC01Pos, DigestC02Pos, DigestC03Unt, DigestC04Unt, DigestC05Unt},
		`{"corpus":"corners","name":"triangle"}`: {DigestBlank, DigestB01Pos, DigestB02Pos, DigestB03Neg, DigestB04Neg},
	}

	var expected []DiffBetweenDigests

	for grouping, digests := range groups {
		// Sort to make sure the digests are in ascending alphabetical order. This way left and right
		// will be appropriately assigned (left should always be < right).
		sort.Slice(digests, func(i, j int) bool {
			return digests[i] < digests[j]
		})
		for leftIdx, leftDigest := range digests {
			leftImg := openNRGBAFromDisk(t, leftDigest)
			for rightIdx := leftIdx + 1; rightIdx < len(digests); rightIdx++ {
				rightDigest := digests[rightIdx]
				require.NotEqual(t, leftDigest, rightDigest, "Somehow there was a duplicate digest")
				rightImg := openNRGBAFromDisk(t, rightDigest)
				dm, _ := diff.PixelDiff(leftImg, rightImg)
				expected = append(expected, DiffBetweenDigests{
					Grouping:    grouping,
					LeftDigest:  leftDigest,
					RightDigest: rightDigest,
					Metrics:     *dm,
				})
			}
		}
	}
	assert.ElementsMatch(t, expected, MakePixelDiffsForCorpusNameGrouping())
}

func openNRGBAFromDisk(t *testing.T, digest types.Digest) *image.NRGBA {
	var img *image.NRGBA
	path := filepath.Join("img", string(digest)+".png")
	err := util.WithReadFile(path, func(r io.Reader) error {
		im, err := png.Decode(r)
		if err != nil {
			return skerr.Wrapf(err, "decoding %s", path)
		}
		img = diff.GetNRGBA(im)
		return nil
	})
	require.NoError(t, err)
	return img
}
