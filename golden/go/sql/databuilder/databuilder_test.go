package databuilder

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

const (
	digestA = types.Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	digestB = types.Digest("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	digestC = types.Digest("cccccccccccccccccccccccccccccccc")
	digestD = types.Digest("dddddddddddddddddddddddddddddddd")
)

func TestGenerateStructs_CalledWithValidInput_ProducesCorrectData(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.Commits().
		Append("author_one", "subject_one", "2020-12-05T16:00:00Z").
		Append("author_two", "subject_two", "2020-12-06T17:00:00Z").
		Append("author_three", "subject_three", "2020-12-07T18:00:00Z").
		Append("author_four", "subject_four", "2020-12-08T19:00:00Z")
	b.UseDigests(map[rune]types.Digest{
		// by convention, upper case are positively triaged, lowercase
		// are untriaged, numbers are negative, symbols are special.
		'A': digestA,
		'b': digestB,
		'1': digestC,
		'D': digestD,
	})
	b.SetGroupingKeys(types.CorpusField, types.PrimaryKeyField)
	b.TracesWithCommonKeys(paramtools.Params{
		"os":              "Android",
		"device":          "Crosshatch",
		"color_mode":      "rgb",
		types.CorpusField: "corpus_one",
	}).History([]string{
		"AAbb",
		"D--D",
	}).Keys([]paramtools.Params{{
		types.PrimaryKeyField: "test_one",
	}, {
		types.PrimaryKeyField: "test_two",
	}}).OptionsAll(paramtools.Params{"ext": "png"}).
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
	}).Keys([]paramtools.Params{{types.PrimaryKeyField: "test_one"}}).
		OptionsPerTrace([]paramtools.Params{{"ext": "png"}}).
		IngestedFrom([]string{"windows_file1", "windows_file2", "windows_file3", ""},
			[]string{"2020-12-11T14:15:00Z", "2020-12-11T15:16:00Z", "2020-12-11T16:17:00Z", ""})

	// TODO(kjlubick) add support for expectations and diff metrics.
	//b.TriageEvent("user_one", "2020-12-12T12:12:12Z").
	//	ExpectationsForGrouping(map[string]string{
	//		types.CorpusField:     "corpus_one",
	//		types.PrimaryKeyField: "test_one"}).
	//	Positive(digestA).
	//	Untriaged(digestB).
	//	Negative()
	//
	//b.ComputeDiffMetricsFromImages("path/to/testdata")

	tables := b.GenerateStructs()
	assert.Equal(t, []schema.OptionsRow{{
		OptionsID: h(`{"ext":"png"}`),
		Keys:      `{"ext":"png"}`,
	}}, tables.Options)
	assert.Equal(t, []schema.GroupingRow{{
		GroupingID: h(`{"name":"test_one","source_type":"corpus_one"}`),
		Keys:       `{"name":"test_one","source_type":"corpus_one"}`,
	}, {
		GroupingID: h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:       `{"name":"test_two","source_type":"corpus_one"}`,
	}}, tables.Groupings)
	assert.Equal(t, []schema.SourceFileRow{{
		SourceFileID: h("crosshatch_file1"),
		SourceFile:   "crosshatch_file1",
		LastIngested: time.Date(2020, time.December, 11, 10, 9, 0, 0, time.UTC),
	}, {
		SourceFileID: h("crosshatch_file2"),
		SourceFile:   "crosshatch_file2",
		LastIngested: time.Date(2020, time.December, 11, 10, 10, 0, 0, time.UTC),
	}, {
		SourceFileID: h("crosshatch_file3"),
		SourceFile:   "crosshatch_file3",
		LastIngested: time.Date(2020, time.December, 11, 10, 11, 0, 0, time.UTC),
	}, {
		SourceFileID: h("crosshatch_file4"),
		SourceFile:   "crosshatch_file4",
		LastIngested: time.Date(2020, time.December, 11, 10, 12, 13, 0, time.UTC),
	}, {
		SourceFileID: h("windows_file1"),
		SourceFile:   "windows_file1",
		LastIngested: time.Date(2020, time.December, 11, 14, 15, 0, 0, time.UTC),
	}, {
		SourceFileID: h("windows_file2"),
		SourceFile:   "windows_file2",
		LastIngested: time.Date(2020, time.December, 11, 15, 16, 0, 0, time.UTC),
	}, {
		SourceFileID: h("windows_file3"),
		SourceFile:   "windows_file3",
		LastIngested: time.Date(2020, time.December, 11, 16, 17, 0, 0, time.UTC),
	}}, tables.SourceFiles)
	assert.Equal(t, []schema.TraceRow{{
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_one","source_type":"corpus_one"}`),
		Keys:                 `{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`,
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:                 `{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`,
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:                 `{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`,
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, tables.Traces)
	assert.Equal(t, []schema.CommitRow{{
		CommitID:    1,
		GitHash:     "0001000100010001000100010001000100010001",
		CommitTime:  time.Date(2020, time.December, 5, 16, 0, 0, 0, time.UTC),
		AuthorEmail: "author_one",
		Subject:     "subject_one",
		HasData:     true,
	}, {
		CommitID:    2,
		GitHash:     "0002000200020002000200020002000200020002",
		CommitTime:  time.Date(2020, time.December, 6, 17, 0, 0, 0, time.UTC),
		AuthorEmail: "author_two",
		Subject:     "subject_two",
		HasData:     true,
	}, {
		CommitID:    3,
		GitHash:     "0003000300030003000300030003000300030003",
		CommitTime:  time.Date(2020, time.December, 7, 18, 0, 0, 0, time.UTC),
		AuthorEmail: "author_three",
		Subject:     "subject_three",
		HasData:     true,
	}, {
		CommitID:    4,
		GitHash:     "0004000400040004000400040004000400040004",
		CommitTime:  time.Date(2020, time.December, 8, 19, 0, 0, 0, time.UTC),
		AuthorEmail: "author_four",
		Subject:     "subject_four",
		HasData:     true,
	}}, tables.Commits)

	pngOptionsID := h(`{"ext":"png"}`)
	testOneGroupingID := h(`{"name":"test_one","source_type":"corpus_one"}`)
	testTwoGroupingID := h(`{"name":"test_two","source_type":"corpus_one"}`)
	assert.Equal(t, []schema.TraceValueRow{{
		Shard:        0x3,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		CommitID:     1,
		Digest:       d(t, digestA),
		GroupingID:   testOneGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file1"),
	}, {
		Shard:        0x3,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		CommitID:     2,
		Digest:       d(t, digestA),
		GroupingID:   testOneGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file2"),
	}, {
		Shard:        0x3,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		CommitID:     3,
		Digest:       d(t, digestB),
		GroupingID:   testOneGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file3"),
	}, {
		Shard:        0x3,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		CommitID:     4,
		Digest:       d(t, digestB),
		GroupingID:   testOneGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file4"),
	}, {
		Shard:        0x4,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		CommitID:     1,
		Digest:       d(t, digestD),
		GroupingID:   testTwoGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file1"),
	}, {
		Shard:        0x4,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		CommitID:     4,
		Digest:       d(t, digestD),
		GroupingID:   testTwoGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file4"),
	}, {
		Shard:        0x6,
		TraceID:      h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		CommitID:     1,
		Digest:       d(t, digestC),
		GroupingID:   testTwoGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("windows_file1"),
	}, {
		Shard:        0x6,
		TraceID:      h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		CommitID:     2,
		Digest:       d(t, digestC),
		GroupingID:   testTwoGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("windows_file2"),
	}, {
		Shard:        0x6,
		TraceID:      h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		CommitID:     3,
		Digest:       d(t, digestD),
		GroupingID:   testTwoGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("windows_file3"),
	}}, tables.TraceValues)
}

func TestCommits_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.Commits()
	assert.Panics(t, func() {
		b.Commits()
	})
}

func TestCommits_InvalidTime_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	assert.Panics(t, func() {
		b.Commits().Append("fine", "dandy", "no good")
	})
}

func TestUseDigests_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	assert.Panics(t, func() {
		b.UseDigests(map[rune]types.Digest{'A': digestA})
	})
}

func TestUseDigests_InvalidData_Panics(t *testing.T) {
	unittest.SmallTest(t)

	assert.Panics(t, func() {
		b := SQLDataBuilder{}
		b.UseDigests(map[rune]types.Digest{'-': digestA})
	})
	assert.Panics(t, func() {
		b := SQLDataBuilder{}
		b.UseDigests(map[rune]types.Digest{'a': "Invalid digest!"})
	})
}

func TestSetGroupingKeys_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys("foo")
	assert.Panics(t, func() {
		b.SetGroupingKeys("bar")
	})
}

func TestTracesWithCommonKeys_MissingSetupCalls_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	assert.Panics(t, func() {
		b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	assert.Panics(t, func() {
		b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
	b.SetGroupingKeys(types.CorpusField)
	assert.Panics(t, func() {
		b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	// Everything should be setup now
	assert.NotPanics(t, func() {
		b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
}

func TestTracesWithCommonKeys_ZeroCommitsSpecified_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits() // oops, no commits
	assert.Panics(t, func() {
		b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
}

func TestHistory_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.History([]string{"A"})
	})
}

func TestHistory_WrongSizeTraces_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	// Expected length is 1
	assert.Panics(t, func() {
		tb.History([]string{"AA"})
	})
	tb = b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.History([]string{"A", "-A"})
	})
	tb = b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.History([]string{"A", ""})
	})
}
func TestHistory_UnknownSymbol_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.History([]string{"?"})
	})
}

func TestKeys_CalledWithoutHistory_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.Keys([]paramtools.Params{{types.CorpusField: "whatever"}})
	})
}

func TestKeys_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	tb.Keys([]paramtools.Params{{types.CorpusField: "whatever"}})
	assert.Panics(t, func() {
		tb.Keys([]paramtools.Params{{types.CorpusField: "whatever"}})
	})
}

func TestKeys_IncorrectLength_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.Keys(nil)
	})
	tb = b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.Keys([]paramtools.Params{})
	})
	tb = b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.Keys([]paramtools.Params{{types.CorpusField: "too"}, {types.CorpusField: "many"}})
	})
}

func TestKeys_MissingGrouping_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys("group1", "group2")
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		// missing group2
		tb.Keys([]paramtools.Params{{"group1": "whatever"}})
	})
}

func TestKeys_IdenticalTraces_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys("group1")
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A", "-"})
	assert.Panics(t, func() {
		tb.Keys([]paramtools.Params{{"group1": "identical"}, {"group1": "identical"}})
	})
}

func TestOptionsPerTrace_CalledWithoutHistory_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.OptionsPerTrace([]paramtools.Params{{"opt": "whatever"}})
	})
}

func TestOptionsPerTrace_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	tb.OptionsPerTrace([]paramtools.Params{{"opt": "whatever"}})
	assert.Panics(t, func() {
		tb.OptionsPerTrace([]paramtools.Params{{"opt": "whatever"}})
	})
}

func TestOptionsPerTrace_IncorrectLength_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.OptionsPerTrace(nil)
	})
	tb = b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.OptionsPerTrace([]paramtools.Params{})
	})
	tb = b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.OptionsPerTrace([]paramtools.Params{{"opt": "too"}, {"opt": "many"}})
	})
}

func TestIngestedFrom_CalledWithoutHistory_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	})
}

func TestIngestedFrom_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	tb.IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	})
}

func TestIngestedFrom_IncorrectLength_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{""})
	})
	tb = b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{""}, []string{"2020-12-05T16:00:00Z"})
	})
	tb = b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{})
	})
	tb = b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{}, []string{"2020-12-05T16:00:00Z"})
	})
}

func TestIngestedFrom_InvalidDateFormat_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{"not valid date"})
	})
}

func TestGenerateStructs_IncompleteData_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.TracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History([]string{"A"})
	assert.Panics(t, func() {
		b.GenerateStructs()
	})
	tb.Keys([]paramtools.Params{{types.CorpusField: "a corpus"}})
	assert.Panics(t, func() {
		b.GenerateStructs()
	})
	tb.OptionsAll(paramtools.Params{"opts": "something"})
	assert.Panics(t, func() {
		b.GenerateStructs()
	})
	tb.IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	assert.NotPanics(t, func() {
		// should be good now
		b.GenerateStructs()
	})
}

func TestGenerateStructs_IdenticalTracesFromTwoSets_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := SQLDataBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.UseDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	b.TracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History([]string{"A"}).
		Keys([]paramtools.Params{{types.CorpusField: "identical"}}).
		OptionsAll(paramtools.Params{"opts": "something"}).
		IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	b.TracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History([]string{"A"}).
		Keys([]paramtools.Params{{types.CorpusField: "identical"}}).
		OptionsAll(paramtools.Params{"opts": "does not impact trace identity"}).
		IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	assert.Panics(t, func() {
		b.GenerateStructs()
	})
}

// h returns the MD5 hash of the provided string.
func h(s string) []byte {
	hash := md5.Sum([]byte(s))
	return hash[:]
}

// d returns the bytes associated with the hex-encoded digest string.
func d(t *testing.T, digest types.Digest) []byte {
	require.Len(t, digest, 2*md5.Size)
	b, err := hex.DecodeString(string(digest))
	require.NoError(t, err)
	return b
}
