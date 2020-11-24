package data_kitchen_sink_test

import (
	"fmt"
	"go.skia.org/infra/golden/go/testutils"
	"image"
	"image/png"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"

	. "go.skia.org/infra/golden/go/testutils/data_kitchen_sink"
)

func TestMakeTraces_CorrectIDs(t *testing.T) {
	unittest.SmallTest(t)
	traces := MakeTraces()
	uniqueIds := map[tiling.TraceID]bool{}
	for _, tp := range traces {
		assert.Equal(t, tiling.TraceIDFromParams(tp.Trace.Keys()), tp.ID)
		assert.NotContains(t, uniqueIds, tp.ID, "traces should be unique - %s was not", tp.ID)
		uniqueIds[tp.ID] = true
	}
}

func TestMakeTraces_CorrectNumberOfDigests(t *testing.T) {
	unittest.SmallTest(t)
	traces := MakeTraces()
	for _, tp := range traces {
		assert.Len(t, tp.Trace.Digests, NumCommits)
		assert.Equal(t, NumCommits, tp.Trace.Len())
	}
}

const diffMetricsTemplate = `{
	LeftDigest: {{.LeftDigestName}}, RightDigest: {{.RightDigestName}},
	Metrics: diff.DiffMetrics{
		NumDiffPixels:    {{.NumPixels}},
		PixelDiffPercent: {{.PixelPercent}},
		MaxRGBADiffs:     [4]int{ {{.R}}, {{.G}}, {{.B}}, {{.A}}},
		CombinedMetric:   {{.CombinedMetric}},
		DimDiffer:        {{.Differ}},
	},
},`

type diffMetricsContex struct {
	LeftDigestName  string
	RightDigestName string
	NumPixels       int
	PixelPercent    float32
	CombinedMetric  float32
	R               int
	G               int
	B               int
	A               int
	Differ          bool
}

var digestToName = map[types.Digest]string{
	DigestBlank:     "DigestBlank",
	DigestB01Pos:    "DigestB01Pos",
	DigestB02Pos:    "DigestB02Pos",
	DigestB03Neg:    "DigestB03Neg",
	DigestB04Neg:    "DigestB04Neg",
	DigestC01Pos:    "DigestC01Pos",
	DigestC02Pos:    "DigestC02Pos",
	DigestC03Unt:    "DigestC03Unt",
	DigestC04Unt:    "DigestC04Unt",
	DigestC05Unt:    "DigestC05Unt",
	DigestC06Pos_CL: "DigestC06Pos_CL",
	DigestC07Unt_CL: "DigestC07Unt_CL",
	DigestA01Pos:    "DigestA01Pos",
	DigestA02Pos:    "DigestA02Pos",
	DigestA03Pos:    "DigestA03Pos",
	DigestA04Unt:    "DigestA04Unt",
	DigestA05Unt:    "DigestA05Unt",
	DigestA06Unt:    "DigestA06Unt",
	DigestA07Pos:    "DigestA07Pos",
	DigestA08Pos:    "DigestA08Pos",
	DigestA09Neg:    "DigestA09Neg",
}

func TestMakeDiffsForCorpusNameGrouping_HasCorrectData(t *testing.T) {
	unittest.SmallTest(t)
	groups := map[string][]types.Digest{
		`{"corpus":"corners","name":"triangle"}`: {DigestBlank, DigestB01Pos, DigestB02Pos, DigestB03Neg, DigestB04Neg},
		`{"corpus":"round","name":"circle"}`:     {DigestC01Pos, DigestC02Pos, DigestC03Unt, DigestC04Unt, DigestC05Unt, DigestC06Pos_CL, DigestC07Unt_CL},
		`{"corpus":"corners","name":"square"}`:   {DigestA01Pos, DigestA02Pos, DigestA03Pos, DigestA04Unt, DigestA05Unt, DigestA06Unt, DigestA07Pos, DigestA08Pos, DigestA09Neg},
	}

	// In case the data does not match, we build up a human-friendly text representation of it and
	// print that if the test fails. This allows us to easily update the expected data if we change
	// an algorithm or an image.
	var formatted strings.Builder
	formatted.WriteString("[]DiffBetweenDigests{\n")
	templ, err := template.New("").Parse(diffMetricsTemplate)
	require.NoError(t, err)
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
				dm := diff.ComputeDiffMetrics(leftImg, rightImg)
				dm.CombinedMetric = testutils.RoundFloat32ToDecimalPlace(dm.CombinedMetric, 5)
				dm.PixelDiffPercent = testutils.RoundFloat32ToDecimalPlace(dm.PixelDiffPercent, 5)
				assert.NotZero(t, dm.NumDiffPixels, "%s and %s aren't different", leftDigest, rightDigest)
				expected = append(expected, DiffBetweenDigests{
					LeftDigest:  leftDigest,
					RightDigest: rightDigest,
					Metrics:     *dm,
				})
				ctx := diffMetricsContex{
					LeftDigestName:  digestToName[leftDigest],
					RightDigestName: digestToName[rightDigest],
					NumPixels:       dm.NumDiffPixels,
					PixelPercent:    dm.PixelDiffPercent,
					CombinedMetric:  dm.CombinedMetric,
					R:               dm.MaxRGBADiffs[0],
					G:               dm.MaxRGBADiffs[1],
					B:               dm.MaxRGBADiffs[2],
					A:               dm.MaxRGBADiffs[3],
					Differ:          dm.DimDiffer,
				}
				require.NoError(t, templ.Execute(&formatted, ctx))
			}
		}
	}
	assert.ElementsMatch(t, expected, MakePixelDiffsForCorpusNameGrouping())
	if t.Failed() {
		formatted.WriteString("\n}\n")
		fmt.Println(formatted.String())
	}
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
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
	for _, ps := range MakePatchsets() {
		assert.NotZero(t, ps.ChangeListID)
		assert.NotZero(t, ps.SystemID)
		assert.NotZero(t, ps.Order)
		assert.NotZero(t, ps.GitHash)
	}
}

func TestMakeChangeLists_DataIsFilledOut(t *testing.T) {
	unittest.SmallTest(t)
	clMap := MakeChangelists()
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
	unittest.SmallTest(t)
	tjd := MakeDataFromTryJobs()
	for _, tj := range tjd {
		assert.NotZero(t, tj.PatchSet)
		assert.NotZero(t, tj.CIS)
		assert.NotEmpty(t, tj.Keys)
		assert.NotEmpty(t, tj.Options)
		assert.NotZero(t, tj.Digest)
	}
}

func TestMakeParamSet_MadeFromAllTraceData(t *testing.T) {
	unittest.SmallTest(t)
	expectedPS := paramtools.ParamSet{}
	for _, tp := range MakeTraces() {
		expectedPS.AddParams(tp.Trace.Keys())
		expectedPS.AddParams(tp.Trace.Options())
	}
	expectedPS.Normalize()
	assert.Equal(t, expectedPS, MakeParamSet())
}

func TestMakeDataFromTryJobs_IsConsistentWithMakeTryjobs(t *testing.T) {
	unittest.SmallTest(t)
	tryjobsByCombinedID := MakeTryjobs()

	// Go through all tryjob results and make sure the tryjob/cis referenced there shows up in
	// the same map of tryjobs above.
	tjResults := MakeDataFromTryJobs()
	for _, tjr := range tjResults {
		tryjobs, ok := tryjobsByCombinedID[tjr.PatchSet]
		require.True(t, ok, "unknown patchset id %v", tjr.PatchSet)
		found := true
		for _, tj := range tryjobs {
			if tj.SystemID == tjr.TryJobID {
				assert.Equal(t, tj.System, tjr.CIS)
				assert.NotZero(t, tj.DisplayName)
				assert.NotZero(t, tj.Updated)
				found = true
			}
		}
		assert.True(t, found, "could not find entry for %s", tjr.TryJobID)
	}
}
