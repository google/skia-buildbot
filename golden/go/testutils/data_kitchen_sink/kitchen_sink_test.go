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
		`{"corpus":"corners","name":"triangle"}`: {DigestBlank, DigestB01Pos, DigestB02Pos, DigestB03Neg, DigestB04Neg},
		`{"corpus":"round","name":"circle"}`:     {DigestC01Pos, DigestC02Pos, DigestC03Unt, DigestC04Unt, DigestC05Unt, DigestC06Pos_CL, DigestC07Unt_CL},
		`{"corpus":"corners","name":"square"}`:   {DigestA01Pos, DigestA02Pos, DigestA03Pos, DigestA04Unt, DigestA05Unt, DigestA06Unt, DigestA07Pos, DigestA08Pos},
	}

	var expected []DiffBetweenDigests

	for _, digests := range groups {
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
				assert.NotZero(t, dm.NumDiffPixels, "%s and %s aren't different", leftDigest, rightDigest)
				expected = append(expected, DiffBetweenDigests{
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

func TestMakeCommits_DataIsFilledOut(t *testing.T) {
	c := MakeCommits()
	assert.Len(t, c, NumCommits)
	// Make sure all the fields are set
	for _, co := range c {
		assert.NotZero(t, co.Subject)
		assert.NotZero(t, co.Hash)
		assert.NotZero(t, co.CommitTime)
		assert.NotZero(t, co.Author)
	}
}

func TestMakePatchSets_DataIsFilledOut(t *testing.T) {
	for _, ps := range MakePatchSets() {
		assert.NotZero(t, ps.ChangeListID)
		assert.NotZero(t, ps.SystemID)
		assert.NotZero(t, ps.Order)
		assert.NotZero(t, ps.GitHash)
	}
}

func TestMakeChangeLists_DataIsFilledOut(t *testing.T) {
	clMap := MakeChangeLists()
	assert.Contains(t, clMap, GerritCRS)
	assert.Contains(t, clMap, GerritInternalCRS)
	for _, cls := range clMap {
		for _, cl := range cls {
			assert.NotZero(t, cl.SystemID)
			assert.NotZero(t, cl.Owner)
			assert.NotZero(t, cl.Subject)
			assert.NotZero(t, cl.Updated)
		}
	}
}

func TestMakeDataFromTryJobs_DataIsFilledOut(t *testing.T) {
	tjd := MakeDataFromTryJobs()
	for _, tj := range tjd {
		assert.NotZero(t, tj.PatchSet)
		assert.NotZero(t, tj.CIS)
		assert.NotEmpty(t, tj.Keys)
		assert.NotEmpty(t, tj.Options)
		assert.NotZero(t, tj.Digest)
	}
}
