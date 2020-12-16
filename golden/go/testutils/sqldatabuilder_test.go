package testutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/types"
)

const (
	digestA = types.Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	digestB = types.Digest("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	digestC = types.Digest("cccccccccccccccccccccccccccccccc")
	digestD = types.Digest("dddddddddddddddddddddddddddddddd")
)

func TestDataBuilder_TwoSetsOfTraces_Success(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.DenseHistory().
		AddCommit("author_one", "subject_one", time.Date(2020, time.December, 5, 16, 0, 0, 0, time.UTC)).
		AddCommit("author_two", "subject_two", time.Date(2020, time.December, 5, 16, 0, 0, 0, time.UTC)).
		AddCommit("author_three", "subject_three", time.Date(2020, time.December, 5, 16, 0, 0, 0, time.UTC)).
		AddCommit("author_four", "subject_four", time.Date(2020, time.December, 5, 16, 0, 0, 0, time.UTC))
	b.UseDigests(map[rune]types.Digest{
		// by convention, upper case are positively triaged, lowercase
		// are untriaged, numbers are negative, symbols are special.
		'A': digestA,
		'b': digestB,
		'1': digestC,
		'D': digestD,
	})
	b.TracesWithCommonKeys(paramtools.Params{
		"os":              "Android",
		"device":          "Crosshatch",
		"color_mode":      "rgb",
		types.CorpusField: "corpus_one",
	}).History([]string{
		"AAbb",
		"D--D",
	}).Keys([]paramtools.Params{
		{
			types.PrimaryKeyField: "test_one",
		}, {
			types.PrimaryKeyField: "test_two",
		},
	}).OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{"crosshatch_file1", "crosshatch_file2", "crosshatch_file3", "crosshatch_file4"},
			[]string{"2020-12-11T10:09:00Z", "2020-12-11T10:10:00Z", "2020-12-11T10:11:00Z", "2020-12-11T10:12:13Z"})

	b.TracesWithCommonKeys(paramtools.Params{
		"os":                  "Windows10.7",
		"device":              "NUC1234",
		"color_mode":          "rgb",
		types.CorpusField:     "corpus_one",
		types.PrimaryKeyField: "test_two",
	}).History([]string{
		"11D-",
	}).Options([]paramtools.Params{{"ext": "png"}}).
		IngestedFrom([]string{"windows_file1", "windows_file2", "windows_file3", ""},
			[]string{"2020-12-11T14:15:00Z", "2020-12-11T15:16:00Z", "2020-12-11T16:17:00Z", ""})

	b.TriageEvent("user_one", "2020-12-12T12:12:12Z").
		ExpectationsForGrouping(map[string]string{
			types.CorpusField:     "corpus_one",
			types.PrimaryKeyField: "test_one"}).
		Positive(digestA).
		Untriaged(digestB).
		Negative()

	b.ComputeDiffMetricsFromImages("path/to/testdata")

	require.NoError(b.Validate())

	tables := b.GenerateStructs()
	assert.NotEmpty(t, tables.Options)
	assert.NotEmpty(t, tables.Groupings)
	assert.NotEmpty(t, tables.TraceValues)
	//assert.ElementsMatch(t, []sql.TraceValueRow{}, tables.TraceValues)
}
