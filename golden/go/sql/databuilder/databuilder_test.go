package databuilder

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
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

func TestBuild_CalledWithValidInput_ProducesCorrectData(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{TileWidth: 3}
	b.CommitsWithData().
		Insert("001", "author_one", "subject_one", "2020-12-05T16:00:00Z").
		Insert("002", "author_two", "subject_two", "2020-12-06T17:00:00Z").
		Insert("003", "author_three", "subject_three", "2020-12-07T18:00:00Z").
		Insert("004", "author_four", "subject_four", "2020-12-08T19:00:00Z")
	b.CommitsWithNoData().
		Insert("005", "5555555555555555555555555555555555555555", "author_five", "no data yet", "2020-12-08T20:00:00Z")
	b.SetDigests(map[rune]types.Digest{
		// by convention, upper case are positively triaged, lowercase
		// are untriaged, numbers are negative, symbols are special.
		'A': digestA,
		'b': digestB,
		'1': digestC,
		'D': digestD,
	})
	b.SetGroupingKeys(types.CorpusField, types.PrimaryKeyField)
	b.AddTracesWithCommonKeys(paramtools.Params{
		"os":              "Android",
		"device":          "Crosshatch",
		"color_mode":      "rgb",
		types.CorpusField: "corpus_one",
	}).History(
		"AAbb",
		"D--D",
	).Keys([]paramtools.Params{{
		types.PrimaryKeyField: "test_one",
	}, {
		types.PrimaryKeyField: "test_two",
	}}).OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{"crosshatch_file1", "crosshatch_file2", "crosshatch_file3", "crosshatch_file4"},
			[]string{"2020-12-11T10:09:00Z", "2020-12-11T10:10:00Z", "2020-12-11T10:11:00Z", "2020-12-11T10:12:13Z"})

	b.AddTracesWithCommonKeys(paramtools.Params{
		"os":                  "Windows10.7",
		"device":              "NUC1234",
		"color_mode":          "rgb",
		types.CorpusField:     "corpus_one",
		types.PrimaryKeyField: "test_two",
	}).History("11D-").
		Keys([]paramtools.Params{{types.PrimaryKeyField: "test_one"}}).
		OptionsPerTrace([]paramtools.Params{{"ext": "png"}}).
		IngestedFrom([]string{"windows_file1", "windows_file2", "windows_file3", ""},
			[]string{"2020-12-11T14:15:00Z", "2020-12-11T15:16:00Z", "2020-12-11T16:17:00Z", ""})

	b.AddTriageEvent("user_one", "2020-12-12T12:12:12Z").
		ExpectationsForGrouping(map[string]string{
			types.CorpusField:     "corpus_one",
			types.PrimaryKeyField: "test_one"}).
		Positive(digestA)
	b.AddTriageEvent("user_two", "2020-12-13T13:13:13Z").
		ExpectationsForGrouping(map[string]string{
			types.CorpusField:     "corpus_one",
			types.PrimaryKeyField: "test_two"}).
		Positive(digestD).
		Negative(digestC)

	firstIgnoreRuleID := b.AddIgnoreRule("ignore_author_one", "ignore_author_two", "2021-03-14T15:09:27Z", "note 1",
		paramtools.ParamSet{
			"does not": []string{"apply", "to any trace"},
		})
	secondIgnoreRuleID := b.AddIgnoreRule("ignore_author_two", "ignore_author_one", "2021-06-28T03:18:53Z", "note 2",
		paramtools.ParamSet{
			"os":     []string{"Windows10.7", "Windows10.8"},
			"device": []string{"NUC1234"},
		})

	dir := testutils.TestDataDir(t)
	b.ComputeDiffMetricsFromImages(dir, "2020-12-14T14:14:14Z")

	tables := b.Build()
	assert.Equal(t, []schema.OptionsRow{{
		OptionsID: h(`{"ext":"png"}`),
		Keys:      paramtools.Params{"ext": "png"},
	}}, tables.Options)
	assert.Equal(t, []schema.GroupingRow{{
		GroupingID: h(`{"name":"test_one","source_type":"corpus_one"}`),
		Keys:       paramtools.Params{"name": "test_one", "source_type": "corpus_one"},
	}, {
		GroupingID: h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:       paramtools.Params{"name": "test_two", "source_type": "corpus_one"},
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
		Keys:                 paramtools.Params{"color_mode": "rgb", "device": "Crosshatch", "name": "test_one", "os": "Android", "source_type": "corpus_one"},
		MatchesAnyIgnoreRule: schema.NBFalse,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:                 paramtools.Params{"color_mode": "rgb", "device": "Crosshatch", "name": "test_two", "os": "Android", "source_type": "corpus_one"},
		MatchesAnyIgnoreRule: schema.NBFalse,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:                 paramtools.Params{"color_mode": "rgb", "device": "NUC1234", "name": "test_two", "os": "Windows10.7", "source_type": "corpus_one"},
		MatchesAnyIgnoreRule: schema.NBTrue,
	}}, tables.Traces)
	assert.Equal(t, []schema.CommitWithDataRow{{
		CommitID: "001",
		TileID:   0,
	}, {
		CommitID: "002",
		TileID:   0,
	}, {
		CommitID: "003",
		TileID:   0,
	}, {
		CommitID: "004",
		TileID:   1,
	}}, tables.CommitsWithData)
	assert.Equal(t, []schema.GitCommitRow{{
		GitHash:     gitHash("001"),
		CommitID:    "001",
		CommitTime:  time.Date(2020, time.December, 5, 16, 0, 0, 0, time.UTC),
		AuthorEmail: "author_one",
		Subject:     "subject_one",
	}, {
		GitHash:     gitHash("002"),
		CommitID:    "002",
		CommitTime:  time.Date(2020, time.December, 6, 17, 0, 0, 0, time.UTC),
		AuthorEmail: "author_two",
		Subject:     "subject_two",
	}, {
		GitHash:     gitHash("003"),
		CommitID:    "003",
		CommitTime:  time.Date(2020, time.December, 7, 18, 0, 0, 0, time.UTC),
		AuthorEmail: "author_three",
		Subject:     "subject_three",
	}, {
		GitHash:     gitHash("004"),
		CommitID:    "004",
		CommitTime:  time.Date(2020, time.December, 8, 19, 0, 0, 0, time.UTC),
		AuthorEmail: "author_four",
		Subject:     "subject_four",
	}, {
		GitHash:     "5555555555555555555555555555555555555555",
		CommitID:    "005",
		CommitTime:  time.Date(2020, time.December, 8, 20, 0, 0, 0, time.UTC),
		AuthorEmail: "author_five",
		Subject:     "no data yet",
	}}, tables.GitCommits)

	pngOptionsID := h(`{"ext":"png"}`)
	testOneGroupingID := h(`{"name":"test_one","source_type":"corpus_one"}`)
	testTwoGroupingID := h(`{"name":"test_two","source_type":"corpus_one"}`)
	assert.Equal(t, []schema.TraceValueRow{{
		Shard:        0x3,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		CommitID:     "001",
		Digest:       d(t, digestA),
		GroupingID:   testOneGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file1"),
	}, {
		Shard:        0x3,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		CommitID:     "002",
		Digest:       d(t, digestA),
		GroupingID:   testOneGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file2"),
	}, {
		Shard:        0x3,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		CommitID:     "003",
		Digest:       d(t, digestB),
		GroupingID:   testOneGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file3"),
	}, {
		Shard:        0x3,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		CommitID:     "004",
		Digest:       d(t, digestB),
		GroupingID:   testOneGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file4"),
	}, {
		Shard:        0x4,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		CommitID:     "001",
		Digest:       d(t, digestD),
		GroupingID:   testTwoGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file1"),
	}, {
		Shard:        0x4,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		CommitID:     "004",
		Digest:       d(t, digestD),
		GroupingID:   testTwoGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("crosshatch_file4"),
	}, {
		Shard:        0x6,
		TraceID:      h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		CommitID:     "001",
		Digest:       d(t, digestC),
		GroupingID:   testTwoGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("windows_file1"),
	}, {
		Shard:        0x6,
		TraceID:      h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		CommitID:     "002",
		Digest:       d(t, digestC),
		GroupingID:   testTwoGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("windows_file2"),
	}, {
		Shard:        0x6,
		TraceID:      h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		CommitID:     "003",
		Digest:       d(t, digestD),
		GroupingID:   testTwoGroupingID,
		OptionsID:    pngOptionsID,
		SourceFileID: h("windows_file3"),
	}}, tables.TraceValues)
	require.Len(t, tables.ExpectationRecords, 2)
	recordIDOne := tables.ExpectationRecords[0].ExpectationRecordID
	recordIDTwo := tables.ExpectationRecords[1].ExpectationRecordID
	assert.Equal(t, []schema.ExpectationRecordRow{{
		ExpectationRecordID: recordIDOne,
		BranchName:          nil, // primary branch
		UserName:            "user_one",
		TriageTime:          time.Date(2020, time.December, 12, 12, 12, 12, 0, time.UTC),
		NumChanges:          1,
	}, {
		ExpectationRecordID: recordIDTwo,
		BranchName:          nil, // primary branch
		UserName:            "user_two",
		TriageTime:          time.Date(2020, time.December, 13, 13, 13, 13, 0, time.UTC),
		NumChanges:          2,
	}}, tables.ExpectationRecords)
	assert.Equal(t, []schema.ExpectationDeltaRow{{
		ExpectationRecordID: recordIDOne,
		GroupingID:          testOneGroupingID,
		Digest:              d(t, digestA),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
	}, {
		ExpectationRecordID: recordIDTwo,
		GroupingID:          testTwoGroupingID,
		Digest:              d(t, digestD),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
	}, {
		ExpectationRecordID: recordIDTwo,
		GroupingID:          testTwoGroupingID,
		Digest:              d(t, digestC),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelNegative,
	}}, tables.ExpectationDeltas)
	assert.Equal(t, []schema.ExpectationRow{{
		GroupingID:          testOneGroupingID,
		Digest:              d(t, digestA),
		Label:               schema.LabelPositive,
		ExpectationRecordID: &recordIDOne,
	}, {
		GroupingID:          testTwoGroupingID,
		Digest:              d(t, digestD),
		Label:               schema.LabelPositive,
		ExpectationRecordID: &recordIDTwo,
	}, {
		GroupingID:          testTwoGroupingID,
		Digest:              d(t, digestC),
		Label:               schema.LabelNegative,
		ExpectationRecordID: &recordIDTwo,
	}, {
		GroupingID:          testOneGroupingID,
		Digest:              d(t, digestB),
		Label:               schema.LabelUntriaged,
		ExpectationRecordID: nil,
	}}, tables.Expectations)
	ts := time.Date(2020, time.December, 14, 14, 14, 14, 0, time.UTC)
	assert.ElementsMatch(t, []schema.DiffMetricRow{{
		LeftDigest:        d(t, digestA),
		RightDigest:       d(t, digestB),
		NumPixelsDiff:     7,
		PercentPixelsDiff: 10.9375,
		MaxRGBADiffs:      [4]int{250, 244, 197, 51},
		MaxChannelDiff:    250,
		CombinedMetric:    2.9445405,
		DimensionsDiffer:  false,
		Timestamp:         ts,
	}, {
		LeftDigest:        d(t, digestB),
		RightDigest:       d(t, digestA),
		NumPixelsDiff:     7,
		PercentPixelsDiff: 10.9375,
		MaxRGBADiffs:      [4]int{250, 244, 197, 51},
		MaxChannelDiff:    250,
		CombinedMetric:    2.9445405,
		DimensionsDiffer:  false,
		Timestamp:         ts,
	}, {
		LeftDigest:        d(t, digestC),
		RightDigest:       d(t, digestD),
		NumPixelsDiff:     36,
		PercentPixelsDiff: 56.25,
		MaxRGBADiffs:      [4]int{106, 21, 21, 0},
		MaxChannelDiff:    106,
		CombinedMetric:    3.4844475,
		DimensionsDiffer:  false,
		Timestamp:         ts,
	}, {
		LeftDigest:        d(t, digestD),
		RightDigest:       d(t, digestC),
		NumPixelsDiff:     36,
		PercentPixelsDiff: 56.25,
		MaxRGBADiffs:      [4]int{106, 21, 21, 0},
		MaxChannelDiff:    106,
		CombinedMetric:    3.4844475,
		DimensionsDiffer:  false,
		Timestamp:         ts,
	}}, tables.DiffMetrics)
	assert.ElementsMatch(t, []schema.TiledTraceDigestRow{{
		TraceID: h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		TileID:  0,
		Digest:  d(t, digestA),
	}, {
		TraceID: h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		TileID:  0,
		Digest:  d(t, digestB),
	}, {
		TraceID: h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		TileID:  0,
		Digest:  d(t, digestD),
	}, {
		TraceID: h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		TileID:  0,
		Digest:  d(t, digestC),
	}, {
		TraceID: h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		TileID:  0,
		Digest:  d(t, digestD),
	}, {
		TraceID: h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		TileID:  1,
		Digest:  d(t, digestB),
	}, {
		TraceID: h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		TileID:  1,
		Digest:  d(t, digestD),
	}}, tables.TiledTraceDigests)
	assert.ElementsMatch(t, []schema.PrimaryBranchParamRow{
		{Key: "name", Value: "test_one", TileID: 0},
		{Key: "name", Value: "test_two", TileID: 0},
		{Key: "device", Value: "Crosshatch", TileID: 0},
		{Key: "device", Value: "NUC1234", TileID: 0},
		{Key: "os", Value: "Android", TileID: 0},
		{Key: "os", Value: "Windows10.7", TileID: 0},
		{Key: "color_mode", Value: "rgb", TileID: 0},
		{Key: "source_type", Value: "corpus_one", TileID: 0},
		{Key: "ext", Value: "png", TileID: 0},
		// Note there's no Windows 10.7 or NUC1234 key because that hasn't been generated in
		// the second tile (starting at commit 4).
		{Key: "name", Value: "test_one", TileID: 1},
		{Key: "name", Value: "test_two", TileID: 1},
		{Key: "device", Value: "Crosshatch", TileID: 1},
		{Key: "os", Value: "Android", TileID: 1},
		{Key: "color_mode", Value: "rgb", TileID: 1},
		{Key: "source_type", Value: "corpus_one", TileID: 1},
		{Key: "ext", Value: "png", TileID: 1},
	}, tables.PrimaryBranchParams)
	assert.ElementsMatch(t, []schema.ValueAtHeadRow{{
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		MostRecentCommitID:   "004",
		Digest:               d(t, digestB),
		OptionsID:            pngOptionsID,
		GroupingID:           testOneGroupingID,
		Corpus:               "corpus_one",
		Keys:                 paramtools.Params{"color_mode": "rgb", "device": "Crosshatch", "name": "test_one", "os": "Android", "source_type": "corpus_one"},
		MatchesAnyIgnoreRule: schema.NBFalse,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		MostRecentCommitID:   "004",
		Digest:               d(t, digestD),
		OptionsID:            pngOptionsID,
		GroupingID:           testTwoGroupingID,
		Corpus:               "corpus_one",
		Keys:                 paramtools.Params{"color_mode": "rgb", "device": "Crosshatch", "name": "test_two", "os": "Android", "source_type": "corpus_one"},
		MatchesAnyIgnoreRule: schema.NBFalse,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		MostRecentCommitID:   "003",
		Digest:               d(t, digestD),
		OptionsID:            pngOptionsID,
		GroupingID:           testTwoGroupingID,
		Corpus:               "corpus_one",
		Keys:                 paramtools.Params{"color_mode": "rgb", "device": "NUC1234", "name": "test_two", "os": "Windows10.7", "source_type": "corpus_one"},
		MatchesAnyIgnoreRule: schema.NBTrue,
	}}, tables.ValuesAtHead)
	assert.Equal(t, []schema.IgnoreRuleRow{{
		IgnoreRuleID: firstIgnoreRuleID,
		CreatorEmail: "ignore_author_one",
		UpdatedEmail: "ignore_author_two",
		Expires:      time.Date(2021, time.March, 14, 15, 9, 27, 0, time.UTC),
		Note:         "note 1",
		Query:        paramtools.ReadOnlyParamSet{"does not": []string{"apply", "to any trace"}},
	}, {
		IgnoreRuleID: secondIgnoreRuleID,
		CreatorEmail: "ignore_author_two",
		UpdatedEmail: "ignore_author_one",
		Expires:      time.Date(2021, time.June, 28, 03, 18, 53, 0, time.UTC),
		Note:         "note 2",
		Query:        paramtools.ReadOnlyParamSet{"device": []string{"NUC1234"}, "os": []string{"Windows10.7", "Windows10.8"}},
	}}, tables.IgnoreRules)
}

func TestBuild_CalledWithChangelistData_ProducesCorrectData(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.CommitsWithData().
		Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	b.SetDigests(map[rune]types.Digest{
		// by convention, upper case are positively triaged, lowercase
		// are untriaged, numbers are negative, symbols are special.
		'A': digestA,
		'b': digestB,
		'1': digestC,
		'D': digestD,
	})
	b.SetGroupingKeys(types.CorpusField, types.PrimaryKeyField)
	b.AddTracesWithCommonKeys(paramtools.Params{
		"os":              "Android",
		"device":          "Crosshatch",
		"color_mode":      "rgb",
		types.CorpusField: "corpus_one",
	}).History(
		"A",
		"D",
	).Keys([]paramtools.Params{{
		types.PrimaryKeyField: "test_one",
	}, {
		types.PrimaryKeyField: "test_two",
	}}).OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{"crosshatch_file1"}, []string{"2020-12-11T10:09:00Z"})

	cl := b.AddChangelist("changelist_one", "gerrit", "owner_one", "First CL", schema.StatusAbandoned)
	cl.AddPatchset("ps_2", "ps_hash_2", 2).
		DataWithCommonKeys(paramtools.Params{
			"os":              "Android",
			"device":          "Crosshatch",
			"color_mode":      "rgb",
			types.CorpusField: "corpus_one",
		}).Digests(digestB, digestC, digestD).
		Keys([]paramtools.Params{{
			types.PrimaryKeyField: "test_one",
		}, {
			types.PrimaryKeyField: "test_two",
		}, {
			types.PrimaryKeyField: "test_three",
		}}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob("tryjob_001", "bb", "Test-Crosshatch", "tryjob_file1", "2020-12-11T10:11:00Z")
	cl.AddPatchset("ps_3", "ps_hash_3", 3).
		DataWithCommonKeys(paramtools.Params{
			"os":              "Android",
			"device":          "Crosshatch",
			"color_mode":      "rgb",
			types.CorpusField: "corpus_one",
		}).Digests(digestB, digestC, digestA).
		Keys([]paramtools.Params{{
			types.PrimaryKeyField: "test_one",
		}, {
			types.PrimaryKeyField: "test_two",
		}, {
			types.PrimaryKeyField: "test_three",
		}}).OptionsPerPoint([]paramtools.Params{
		{"ext": "png"},
		{"ext": "png"},
		{"ext": "png", "matcher": "fuzzy", "threshold": "2"},
	}).
		FromTryjob("tryjob_002", "bb", "Test-Crosshatch", "tryjob_file2", "2020-12-11T11:12:13Z")

	cl.AddTriageEvent("cl_user", "2020-12-11T11:40:00Z").
		ExpectationsForGrouping(map[string]string{
			types.CorpusField:     "corpus_one",
			types.PrimaryKeyField: "test_three"}).
		Negative(digestD)
	b.AddTriageEvent("user_one", "2020-12-12T12:12:12Z").
		ExpectationsForGrouping(map[string]string{
			types.CorpusField:     "corpus_one",
			types.PrimaryKeyField: "test_one"}).
		Positive(digestA).
		ExpectationsForGrouping(map[string]string{
			types.CorpusField:     "corpus_one",
			types.PrimaryKeyField: "test_two"}).
		Positive(digestD)

	b.AddIgnoreRule("ignore_author", "ignore_author", "2021-03-14T15:09:27Z", "note 1",
		paramtools.ParamSet{
			types.PrimaryKeyField: []string{"test_two"},
		})

	dir := testutils.TestDataDir(t)
	b.ComputeDiffMetricsFromImages(dir, "2020-12-14T14:14:14Z")

	tables := b.Build()
	assert.Equal(t, []schema.OptionsRow{{
		OptionsID: h(`{"ext":"png"}`),
		Keys:      paramtools.Params{"ext": "png"},
	}, {
		OptionsID: h(`{"ext":"png","matcher":"fuzzy","threshold":"2"}`),
		Keys:      paramtools.Params{"ext": "png", "matcher": "fuzzy", "threshold": "2"},
	}}, tables.Options)
	assert.Equal(t, []schema.GroupingRow{{
		GroupingID: h(`{"name":"test_one","source_type":"corpus_one"}`),
		Keys:       paramtools.Params{"name": "test_one", "source_type": "corpus_one"},
	}, {
		GroupingID: h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:       paramtools.Params{"name": "test_two", "source_type": "corpus_one"},
	}, {
		GroupingID: h(`{"name":"test_three","source_type":"corpus_one"}`),
		Keys:       paramtools.Params{"name": "test_three", "source_type": "corpus_one"},
	}}, tables.Groupings)
	assert.Equal(t, []schema.SourceFileRow{{
		SourceFileID: h("crosshatch_file1"),
		SourceFile:   "crosshatch_file1",
		LastIngested: time.Date(2020, time.December, 11, 10, 9, 0, 0, time.UTC),
	}, {
		SourceFileID: h("tryjob_file1"),
		SourceFile:   "tryjob_file1",
		LastIngested: time.Date(2020, time.December, 11, 10, 11, 0, 0, time.UTC),
	}, {
		SourceFileID: h("tryjob_file2"),
		SourceFile:   "tryjob_file2",
		LastIngested: time.Date(2020, time.December, 11, 11, 12, 13, 0, time.UTC),
	}}, tables.SourceFiles)
	assert.Equal(t, []schema.TraceRow{{
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_one","source_type":"corpus_one"}`),
		Keys:                 paramtools.Params{"color_mode": "rgb", "device": "Crosshatch", "name": "test_one", "os": "Android", "source_type": "corpus_one"},
		MatchesAnyIgnoreRule: schema.NBFalse,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:                 paramtools.Params{"color_mode": "rgb", "device": "Crosshatch", "name": "test_two", "os": "Android", "source_type": "corpus_one"},
		MatchesAnyIgnoreRule: schema.NBTrue,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_three","os":"Android","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_three","source_type":"corpus_one"}`),
		Keys:                 paramtools.Params{"color_mode": "rgb", "device": "Crosshatch", "name": "test_three", "os": "Android", "source_type": "corpus_one"},
		MatchesAnyIgnoreRule: schema.NBFalse,
	}}, tables.Traces)
	assert.Equal(t, []schema.TraceValueRow{{
		Shard:        0x3,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		CommitID:     "123",
		Digest:       d(t, digestA),
		GroupingID:   h(`{"name":"test_one","source_type":"corpus_one"}`),
		OptionsID:    h(`{"ext":"png"}`),
		SourceFileID: h("crosshatch_file1"),
	}, {
		Shard:        0x4,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		CommitID:     "123",
		Digest:       d(t, digestD),
		GroupingID:   h(`{"name":"test_two","source_type":"corpus_one"}`),
		OptionsID:    h(`{"ext":"png"}`),
		SourceFileID: h("crosshatch_file1"),
	}}, tables.TraceValues)
	assert.ElementsMatch(t, []schema.PrimaryBranchParamRow{
		{Key: "name", Value: "test_one", TileID: 0},
		{Key: "name", Value: "test_two", TileID: 0},
		{Key: "device", Value: "Crosshatch", TileID: 0},
		{Key: "os", Value: "Android", TileID: 0},
		{Key: "color_mode", Value: "rgb", TileID: 0},
		{Key: "source_type", Value: "corpus_one", TileID: 0},
		{Key: "ext", Value: "png", TileID: 0},
	}, tables.PrimaryBranchParams)
	qualifiedCLID := "gerrit_changelist_one"
	assert.Equal(t, []schema.ChangelistRow{{
		ChangelistID:     qualifiedCLID,
		System:           "gerrit",
		Status:           schema.StatusAbandoned,
		OwnerEmail:       "owner_one",
		Subject:          "First CL",
		LastIngestedData: time.Date(2020, time.December, 11, 11, 12, 13, 0, time.UTC),
	}}, tables.Changelists)
	assert.Equal(t, []schema.PatchsetRow{{
		PatchsetID:   "gerrit_ps_2",
		System:       "gerrit",
		ChangelistID: qualifiedCLID,
		Order:        2,
		GitHash:      "ps_hash_2",
	}, {
		PatchsetID:   "gerrit_ps_3",
		System:       "gerrit",
		ChangelistID: qualifiedCLID,
		Order:        3,
		GitHash:      "ps_hash_3",
	}}, tables.Patchsets)
	assert.Equal(t, []schema.TryjobRow{{
		TryjobID:         "bb_tryjob_001",
		System:           "bb",
		ChangelistID:     qualifiedCLID,
		PatchsetID:       "gerrit_ps_2",
		DisplayName:      "Test-Crosshatch",
		LastIngestedData: time.Date(2020, time.December, 11, 10, 11, 0, 0, time.UTC),
	}, {
		TryjobID:         "bb_tryjob_002",
		System:           "bb",
		ChangelistID:     qualifiedCLID,
		PatchsetID:       "gerrit_ps_3",
		DisplayName:      "Test-Crosshatch",
		LastIngestedData: time.Date(2020, time.December, 11, 11, 12, 13, 0, time.UTC),
	}}, tables.Tryjobs)
	assert.ElementsMatch(t, []schema.SecondaryBranchParamRow{
		{Key: "name", Value: "test_one", BranchName: qualifiedCLID, VersionName: "gerrit_ps_2"},
		{Key: "name", Value: "test_two", BranchName: qualifiedCLID, VersionName: "gerrit_ps_2"},
		{Key: "name", Value: "test_three", BranchName: qualifiedCLID, VersionName: "gerrit_ps_2"},
		{Key: "device", Value: "Crosshatch", BranchName: qualifiedCLID, VersionName: "gerrit_ps_2"},
		{Key: "os", Value: "Android", BranchName: qualifiedCLID, VersionName: "gerrit_ps_2"},
		{Key: "color_mode", Value: "rgb", BranchName: qualifiedCLID, VersionName: "gerrit_ps_2"},
		{Key: "source_type", Value: "corpus_one", BranchName: qualifiedCLID, VersionName: "gerrit_ps_2"},
		{Key: "ext", Value: "png", BranchName: qualifiedCLID, VersionName: "gerrit_ps_2"},

		{Key: "name", Value: "test_one", BranchName: qualifiedCLID, VersionName: "gerrit_ps_3"},
		{Key: "name", Value: "test_two", BranchName: qualifiedCLID, VersionName: "gerrit_ps_3"},
		{Key: "name", Value: "test_three", BranchName: qualifiedCLID, VersionName: "gerrit_ps_3"},
		{Key: "device", Value: "Crosshatch", BranchName: qualifiedCLID, VersionName: "gerrit_ps_3"},
		{Key: "os", Value: "Android", BranchName: qualifiedCLID, VersionName: "gerrit_ps_3"},
		{Key: "color_mode", Value: "rgb", BranchName: qualifiedCLID, VersionName: "gerrit_ps_3"},
		{Key: "source_type", Value: "corpus_one", BranchName: qualifiedCLID, VersionName: "gerrit_ps_3"},
		{Key: "ext", Value: "png", BranchName: qualifiedCLID, VersionName: "gerrit_ps_3"},
		{Key: "matcher", Value: "fuzzy", BranchName: qualifiedCLID, VersionName: "gerrit_ps_3"},
		{Key: "threshold", Value: "2", BranchName: qualifiedCLID, VersionName: "gerrit_ps_3"},
	}, tables.SecondaryBranchParams)
	assert.Equal(t, []schema.SecondaryBranchValueRow{{
		BranchName:   qualifiedCLID,
		VersionName:  "gerrit_ps_2",
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		Digest:       d(t, digestB),
		GroupingID:   h(`{"name":"test_one","source_type":"corpus_one"}`),
		OptionsID:    h(`{"ext":"png"}`),
		SourceFileID: h("tryjob_file1"),
		TryjobID:     "bb_tryjob_001",
	}, {
		BranchName:   qualifiedCLID,
		VersionName:  "gerrit_ps_2",
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		Digest:       d(t, digestC),
		GroupingID:   h(`{"name":"test_two","source_type":"corpus_one"}`),
		OptionsID:    h(`{"ext":"png"}`),
		SourceFileID: h("tryjob_file1"),
		TryjobID:     "bb_tryjob_001",
	}, {
		BranchName:   qualifiedCLID,
		VersionName:  "gerrit_ps_2",
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_three","os":"Android","source_type":"corpus_one"}`),
		Digest:       d(t, digestD),
		GroupingID:   h(`{"name":"test_three","source_type":"corpus_one"}`),
		OptionsID:    h(`{"ext":"png"}`),
		SourceFileID: h("tryjob_file1"),
		TryjobID:     "bb_tryjob_001",
	}, {
		BranchName:   qualifiedCLID,
		VersionName:  "gerrit_ps_3",
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		Digest:       d(t, digestB),
		GroupingID:   h(`{"name":"test_one","source_type":"corpus_one"}`),
		OptionsID:    h(`{"ext":"png"}`),
		SourceFileID: h("tryjob_file2"),
		TryjobID:     "bb_tryjob_002",
	}, {
		BranchName:   qualifiedCLID,
		VersionName:  "gerrit_ps_3",
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		Digest:       d(t, digestC),
		GroupingID:   h(`{"name":"test_two","source_type":"corpus_one"}`),
		OptionsID:    h(`{"ext":"png"}`),
		SourceFileID: h("tryjob_file2"),
		TryjobID:     "bb_tryjob_002",
	}, {
		BranchName:   qualifiedCLID,
		VersionName:  "gerrit_ps_3",
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_three","os":"Android","source_type":"corpus_one"}`),
		Digest:       d(t, digestA),
		GroupingID:   h(`{"name":"test_three","source_type":"corpus_one"}`),
		OptionsID:    h(`{"ext":"png","matcher":"fuzzy","threshold":"2"}`),
		SourceFileID: h("tryjob_file2"),
		TryjobID:     "bb_tryjob_002",
	}}, tables.SecondaryBranchValues)
	require.Len(t, tables.ExpectationRecords, 2)
	primaryBranchRecordID := tables.ExpectationRecords[0].ExpectationRecordID
	clRecordID := tables.ExpectationRecords[1].ExpectationRecordID
	assert.Equal(t, []schema.ExpectationRecordRow{{
		ExpectationRecordID: primaryBranchRecordID,
		BranchName:          nil, // primary branch
		UserName:            "user_one",
		TriageTime:          time.Date(2020, time.December, 12, 12, 12, 12, 0, time.UTC),
		NumChanges:          2,
	}, {
		ExpectationRecordID: clRecordID,
		BranchName:          &qualifiedCLID,
		UserName:            "cl_user",
		TriageTime:          time.Date(2020, time.December, 11, 11, 40, 0, 0, time.UTC),
		NumChanges:          1,
	}}, tables.ExpectationRecords)
	assert.Equal(t, []schema.ExpectationDeltaRow{{
		ExpectationRecordID: primaryBranchRecordID,
		GroupingID:          h(`{"name":"test_one","source_type":"corpus_one"}`),
		Digest:              d(t, digestA),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
	}, {
		ExpectationRecordID: primaryBranchRecordID,
		GroupingID:          h(`{"name":"test_two","source_type":"corpus_one"}`),
		Digest:              d(t, digestD),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
	}, {
		ExpectationRecordID: clRecordID,
		GroupingID:          h(`{"name":"test_three","source_type":"corpus_one"}`),
		Digest:              d(t, digestD),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelNegative,
	}}, tables.ExpectationDeltas)
	assert.Equal(t, []schema.SecondaryBranchExpectationRow{{
		BranchName:          qualifiedCLID,
		GroupingID:          h(`{"name":"test_three","source_type":"corpus_one"}`),
		Digest:              d(t, digestD),
		Label:               schema.LabelNegative,
		ExpectationRecordID: clRecordID,
	}}, tables.SecondaryBranchExpectations)
	assert.Equal(t, []schema.ExpectationRow{{
		GroupingID:          h(`{"name":"test_one","source_type":"corpus_one"}`),
		Digest:              d(t, digestA),
		Label:               schema.LabelPositive,
		ExpectationRecordID: &primaryBranchRecordID,
	}, {
		GroupingID:          h(`{"name":"test_two","source_type":"corpus_one"}`),
		Digest:              d(t, digestD),
		Label:               schema.LabelPositive,
		ExpectationRecordID: &primaryBranchRecordID,
	}}, tables.Expectations)
	ts := time.Date(2020, time.December, 14, 14, 14, 14, 0, time.UTC)
	assert.ElementsMatch(t, []schema.DiffMetricRow{{
		// These first 4 are the same as the first test.
		LeftDigest:        d(t, digestA),
		RightDigest:       d(t, digestB),
		NumPixelsDiff:     7,
		PercentPixelsDiff: 10.9375,
		MaxRGBADiffs:      [4]int{250, 244, 197, 51},
		MaxChannelDiff:    250,
		CombinedMetric:    2.9445405,
		DimensionsDiffer:  false,
		Timestamp:         ts,
	}, {
		LeftDigest:        d(t, digestB),
		RightDigest:       d(t, digestA),
		NumPixelsDiff:     7,
		PercentPixelsDiff: 10.9375,
		MaxRGBADiffs:      [4]int{250, 244, 197, 51},
		MaxChannelDiff:    250,
		CombinedMetric:    2.9445405,
		DimensionsDiffer:  false,
		Timestamp:         ts,
	}, {
		LeftDigest:        d(t, digestC),
		RightDigest:       d(t, digestD),
		NumPixelsDiff:     36,
		PercentPixelsDiff: 56.25,
		MaxRGBADiffs:      [4]int{106, 21, 21, 0},
		MaxChannelDiff:    106,
		CombinedMetric:    3.4844475,
		DimensionsDiffer:  false,
		Timestamp:         ts,
	}, {
		LeftDigest:        d(t, digestD),
		RightDigest:       d(t, digestC),
		NumPixelsDiff:     36,
		PercentPixelsDiff: 56.25,
		MaxRGBADiffs:      [4]int{106, 21, 21, 0},
		MaxChannelDiff:    106,
		CombinedMetric:    3.4844475,
		DimensionsDiffer:  false,
		Timestamp:         ts,
	}, { // The following 2 were calculated on the new test introduced by this CL
		LeftDigest:        d(t, digestA),
		RightDigest:       d(t, digestD),
		NumPixelsDiff:     64,
		PercentPixelsDiff: 100,
		MaxRGBADiffs:      [4]int{250, 244, 197, 255},
		MaxChannelDiff:    255,
		CombinedMetric:    9.653383,
		DimensionsDiffer:  false,
		Timestamp:         ts,
	}, {
		LeftDigest:        d(t, digestD),
		RightDigest:       d(t, digestA),
		NumPixelsDiff:     64,
		PercentPixelsDiff: 100,
		MaxRGBADiffs:      [4]int{250, 244, 197, 255},
		MaxChannelDiff:    255,
		CombinedMetric:    9.653383,
		DimensionsDiffer:  false,
		Timestamp:         ts,
	}}, tables.DiffMetrics)
}

func TestCommits_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.CommitsWithData()
	assert.Panics(t, func() {
		b.CommitsWithData()
	})
}

func TestCommits_InvalidTime_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	assert.Panics(t, func() {
		b.CommitsWithData().Insert("fine", "dandy", "bueno", "no good")
	})
}

func TestCommits_InsertInAnyOrder_Success(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{TileWidth: 2}
	b.CommitsWithData().
		Insert("0100", "author_100", "subject_100", "2021-01-01T01:01:00Z").
		Insert("0099", "author_one", "subject_99", "2020-12-05T15:00:00Z").
		Insert("0098", "author_two", "subject_98", "2020-12-05T14:00:00Z").
		Insert("2000", "author_2k", "subject_2k", "2022-02-02T02:02:00Z")

	b.CommitsWithNoData().
		Insert("1900", "4444444444444444444444444444444444444444", "somebody", "no data 1900", "2021-02-03T04:05:06Z").
		Insert("1850", "3333333333333333333333333333333333333333", "somebody", "no data 1850", "2021-02-03T04:05:00Z")

	tables := b.Build()
	assert.Equal(t, []schema.CommitWithDataRow{{
		CommitID: "0098",
		TileID:   0,
	}, {
		CommitID: "0099",
		TileID:   0,
	}, {
		CommitID: "0100",
		TileID:   1,
	}, {
		CommitID: "2000",
		TileID:   1,
	}}, tables.CommitsWithData)
	assert.Equal(t, []schema.GitCommitRow{{
		GitHash:     gitHash("0098"),
		CommitID:    "0098",
		CommitTime:  time.Date(2020, time.December, 5, 14, 0, 0, 0, time.UTC),
		AuthorEmail: "author_two",
		Subject:     "subject_98",
	}, {
		GitHash:     gitHash("0099"),
		CommitID:    "0099",
		CommitTime:  time.Date(2020, time.December, 5, 15, 0, 0, 0, time.UTC),
		AuthorEmail: "author_one",
		Subject:     "subject_99",
	}, {
		GitHash:     gitHash("0100"),
		CommitID:    "0100",
		CommitTime:  time.Date(2021, time.January, 1, 1, 1, 0, 0, time.UTC),
		AuthorEmail: "author_100",
		Subject:     "subject_100",
	}, {
		GitHash:     "3333333333333333333333333333333333333333",
		CommitID:    "1850",
		CommitTime:  time.Date(2021, time.February, 3, 4, 5, 0, 0, time.UTC),
		AuthorEmail: "somebody",
		Subject:     "no data 1850",
	}, {
		GitHash:     "4444444444444444444444444444444444444444",
		CommitID:    "1900",
		CommitTime:  time.Date(2021, time.February, 3, 4, 5, 6, 0, time.UTC),
		AuthorEmail: "somebody",
		Subject:     "no data 1900",
	}, {
		GitHash:     gitHash("2000"),
		CommitID:    "2000",
		CommitTime:  time.Date(2022, time.February, 2, 2, 2, 0, 0, time.UTC),
		AuthorEmail: "author_2k",
		Subject:     "subject_2k",
	}}, tables.GitCommits)
}

func TestSetDigests_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	assert.Panics(t, func() {
		b.SetDigests(map[rune]types.Digest{'A': digestA})
	})
}

func TestSetDigests_InvalidData_Panics(t *testing.T) {
	unittest.SmallTest(t)

	assert.Panics(t, func() {
		b := TablesBuilder{}
		b.SetDigests(map[rune]types.Digest{'-': digestA})
	})
	assert.Panics(t, func() {
		b := TablesBuilder{}
		b.SetDigests(map[rune]types.Digest{'a': "Invalid digest!"})
	})
}

func TestSetGroupingKeys_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys("foo")
	assert.Panics(t, func() {
		b.SetGroupingKeys("bar")
	})
}

func TestAddTracesWithCommonKeys_MissingSetupCalls_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	assert.Panics(t, func() {
		b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	assert.Panics(t, func() {
		b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
	b.SetGroupingKeys(types.CorpusField)
	assert.Panics(t, func() {
		b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	// Everything should be setup now
	assert.NotPanics(t, func() {
		b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
}

func TestAddTracesWithCommonKeys_ZeroCommitsSpecified_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData() // oops, no commits
	assert.Panics(t, func() {
		b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
}

func TestHistory_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.History("A")
	})
}

func TestHistory_WrongSizeTraces_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	// Expected length is 1
	assert.Panics(t, func() {
		tb.History("AA")
	})
	tb = b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.History("A", "-A")
	})
	tb = b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.History("A", "")
	})
}
func TestHistory_UnknownSymbol_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.History("?")
	})
}

func TestKeys_CalledWithoutHistory_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.Keys([]paramtools.Params{{types.CorpusField: "whatever"}})
	})
}

func TestKeys_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	tb.Keys([]paramtools.Params{{types.CorpusField: "whatever"}})
	assert.Panics(t, func() {
		tb.Keys([]paramtools.Params{{types.CorpusField: "whatever"}})
	})
}

func TestKeys_IncorrectLength_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.Keys(nil)
	})
	tb = b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.Keys([]paramtools.Params{})
	})
	tb = b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.Keys([]paramtools.Params{{types.CorpusField: "too"}, {types.CorpusField: "many"}})
	})
}

func TestKeys_MissingGrouping_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys("group1", "group2")
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		// missing group2
		tb.Keys([]paramtools.Params{{"group1": "whatever"}})
	})
}

func TestKeys_IdenticalTraces_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys("group1")
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A", "-")
	assert.Panics(t, func() {
		tb.Keys([]paramtools.Params{{"group1": "identical"}, {"group1": "identical"}})
	})
}

func TestOptionsPerTrace_CalledWithoutHistory_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.OptionsPerTrace([]paramtools.Params{{"opt": "whatever"}})
	})
}

func TestOptionsPerTrace_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	tb.OptionsPerTrace([]paramtools.Params{{"opt": "whatever"}})
	assert.Panics(t, func() {
		tb.OptionsPerTrace([]paramtools.Params{{"opt": "whatever"}})
	})
}

func TestOptionsPerTrace_IncorrectLength_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.OptionsPerTrace(nil)
	})
	tb = b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.OptionsPerTrace([]paramtools.Params{})
	})
	tb = b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.OptionsPerTrace([]paramtools.Params{{"opt": "too"}, {"opt": "many"}})
	})
}

func TestOptionsPerPoint_CorrectDataLinedUp(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys("test")
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().
		Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z").
		Insert("128", "author_one", "subject_two", "2020-12-05T17:00:00Z")
	b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History("AA", "AA").
		Keys([]paramtools.Params{{"test": "one"}, {"test": "two"}}).
		OptionsPerPoint([][]paramtools.Params{
			{{"option": "cell 1"}, {"option": "cell 2"}},
			{{"option": "cell 3"}, {"option": "cell 4"}},
		}).IngestedFrom([]string{"first", "second"}, []string{"2020-12-05T16:30:00Z", "2020-12-05T17:30:00Z"})
	data := b.Build()
	assert.Equal(t, []schema.OptionsRow{{
		OptionsID: h(`{"option":"cell 1"}`),
		Keys:      paramtools.Params{"option": "cell 1"},
	}, {
		OptionsID: h(`{"option":"cell 2"}`),
		Keys:      paramtools.Params{"option": "cell 2"},
	}, {
		OptionsID: h(`{"option":"cell 3"}`),
		Keys:      paramtools.Params{"option": "cell 3"},
	}, {
		OptionsID: h(`{"option":"cell 4"}`),
		Keys:      paramtools.Params{"option": "cell 4"},
	}}, data.Options)
	assert.Equal(t, schema.OptionsID(h(`{"option":"cell 1"}`)), data.TraceValues[0].OptionsID)
	assert.Equal(t, schema.OptionsID(h(`{"option":"cell 2"}`)), data.TraceValues[1].OptionsID)
	assert.Equal(t, schema.OptionsID(h(`{"option":"cell 3"}`)), data.TraceValues[2].OptionsID)
	assert.Equal(t, schema.OptionsID(h(`{"option":"cell 4"}`)), data.TraceValues[3].OptionsID)
}

func TestIngestedFrom_CalledWithoutHistory_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	})
}

func TestIngestedFrom_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	tb.IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	})
}

func TestIngestedFrom_IncorrectLength_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{""})
	})
	tb = b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{""}, []string{"2020-12-05T16:00:00Z"})
	})
	tb = b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{})
	})
	tb = b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{}, []string{"2020-12-05T16:00:00Z"})
	})
}

func TestIngestedFrom_InvalidDateFormat_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{"not valid date"})
	})
}

func TestBuild_IncompleteData_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		b.Build()
	})
	tb.Keys([]paramtools.Params{{types.CorpusField: "a corpus"}})
	assert.Panics(t, func() {
		b.Build()
	})
	tb.OptionsAll(paramtools.Params{"opts": "something"})
	assert.Panics(t, func() {
		b.Build()
	})
	tb.IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	assert.NotPanics(t, func() {
		// should be good now
		b.Build()
	})
}

func TestBuild_IdenticalTracesFromTwoSets_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History("A").
		Keys([]paramtools.Params{{types.CorpusField: "identical"}}).
		OptionsAll(paramtools.Params{"opts": "something"}).
		IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History("A").
		Keys([]paramtools.Params{{types.CorpusField: "identical"}}).
		OptionsAll(paramtools.Params{"opts": "does not impact trace identity"}).
		IngestedFrom([]string{"file1"}, []string{"2020-12-05T16:00:00Z"})
	assert.Panics(t, func() {
		b.Build()
	})
}

func TestAddTriageEvent_NoGroupingKeys_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	assert.Panics(t, func() {
		b.AddTriageEvent("user", "2020-12-05T16:00:00Z")
	})
}

func TestAddTriageEvent_InvalidTime_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	assert.Panics(t, func() {
		b.AddTriageEvent("user", "invalid time")
	})
}

func TestTriage_ReplacingPreviousExpectations_LabelAndRecordOverwritten(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys("test")
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History("A").Keys([]paramtools.Params{{"test": "one"}}).
		OptionsAll(paramtools.Params{"opt": "opt"}).
		IngestedFrom([]string{"first"}, []string{"2020-12-05T16:30:00Z"})

	b.AddTriageEvent("mistake_user", "2020-12-05T16:00:00Z").
		ExpectationsForGrouping(paramtools.Params{"test": "one"}).
		Positive(digestA)
	b.AddTriageEvent("mistake_user", "2020-12-05T16:00:05Z").
		ExpectationsForGrouping(paramtools.Params{"test": "one"}).
		Triage(digestA, schema.LabelPositive, schema.LabelNegative)
	data := b.Build()
	require.Len(t, data.ExpectationRecords, 2)
	firstID := data.ExpectationRecords[0].ExpectationRecordID
	secondID := data.ExpectationRecords[1].ExpectationRecordID
	assert.Equal(t, []schema.ExpectationRow{{
		GroupingID:          h(`{"test":"one"}`),
		Digest:              d(t, digestA),
		Label:               schema.LabelNegative,
		ExpectationRecordID: &secondID,
	}}, data.Expectations)
	assert.Equal(t, []schema.ExpectationDeltaRow{{
		ExpectationRecordID: firstID,
		GroupingID:          h(`{"test":"one"}`),
		Digest:              d(t, digestA),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelPositive,
	}, {
		ExpectationRecordID: secondID,
		GroupingID:          h(`{"test":"one"}`),
		Digest:              d(t, digestA),
		LabelBefore:         schema.LabelPositive,
		LabelAfter:          schema.LabelNegative,
	}}, data.ExpectationDeltas)
}

func TestExpectationsForGrouping_KeyMissingFromGrouping_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	eb := b.AddTriageEvent("user", "2020-12-05T16:00:00Z")
	assert.Panics(t, func() {
		eb.ExpectationsForGrouping(paramtools.Params{"oops": "missing"})
	})
}

func TestExpectationsBuilderPositive_InvalidDigest_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	eb := b.AddTriageEvent("user", "2020-12-05T16:00:00Z").
		ExpectationsForGrouping(paramtools.Params{types.CorpusField: "whatever"})
	assert.Panics(t, func() {
		eb.Positive("invalid")
	})
}

func TestExpectationsBuilderNegative_InvalidDigest_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	eb := b.AddTriageEvent("user", "2020-12-05T16:00:00Z").
		ExpectationsForGrouping(paramtools.Params{types.CorpusField: "whatever"})
	assert.Panics(t, func() {
		eb.Negative("invalid")
	})
}

func TestComputeDiffMetricsFromImages_IncompleteData_Panics(t *testing.T) {
	unittest.SmallTest(t)
	testDir := testutils.TestDataDir(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	assert.Panics(t, func() {
		b.ComputeDiffMetricsFromImages(testDir, "2020-12-05T16:00:00Z")
	})
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	assert.Panics(t, func() {
		b.ComputeDiffMetricsFromImages(testDir, "2020-12-05T16:00:00Z")
	})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History("A")
	// We should have the right data now.
	assert.NotPanics(t, func() {
		b.ComputeDiffMetricsFromImages(testDir, "2020-12-05T16:00:00Z")
	})
}

func TestComputeDiffMetricsFromImages_InvalidTime_Panics(t *testing.T) {
	unittest.SmallTest(t)
	testDir := testutils.TestDataDir(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History("A")
	assert.Panics(t, func() {
		b.ComputeDiffMetricsFromImages(testDir, "not a valid time")
	})
}

func TestComputeDiffMetricsFromImages_InvalidDirectory_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History("A")
	assert.Panics(t, func() {
		b.ComputeDiffMetricsFromImages("Not a valid directory", "2020-12-05T16:00:00Z")
	})
}

func TestAddIgnoreRule_InvalidDate_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	assert.Panics(t, func() {
		b.AddIgnoreRule("fine", "fine", "Invalid date", "whatever",
			paramtools.ParamSet{"what": []string{"ever"}})
	})
}

func TestAddIgnoreRule_InvalidQuery_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	assert.Panics(t, func() {
		b.AddIgnoreRule("fine", "fine", "2020-12-05T16:00:00Z", "whatever", nil)
	})
	assert.Panics(t, func() {
		b.AddIgnoreRule("fine", "fine", "2020-12-05T16:00:00Z", "whatever", paramtools.ParamSet{})
	})
}

func TestPatchsetBuilder_DataWithCommonKeysChained_Success(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys("test")
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History("A").Keys([]paramtools.Params{{"test": "one"}}).
		OptionsAll(paramtools.Params{"opt": "opt"}).
		IngestedFrom([]string{"first"}, []string{"2020-12-05T16:30:00Z"})

	cl := b.AddChangelist("cl1", "gerrit", "user1", "whatever", schema.StatusOpen)
	ps := cl.AddPatchset("ps1", "ffff111111111111111111111111111111111111", 1)
	ps.DataWithCommonKeys(paramtools.Params{"os": "Android"}).
		Digests(digestB).Keys([]paramtools.Params{{"test": "one"}}).
		OptionsAll(paramtools.Params{"opt": "opt"}).
		FromTryjob("tryjob1", "bb", "TRYJOB1", "tryjob1.txt", "2020-12-05T16:30:00Z")

	ps.DataWithCommonKeys(paramtools.Params{"os": "Mac"}).
		Digests(digestC).Keys([]paramtools.Params{{"test": "one"}}).
		OptionsAll(paramtools.Params{"opt": "opt"}).
		FromTryjob("tryjob2", "bb", "TRYJOB2", "tryjob2.txt", "2020-12-05T16:30:00Z")

	data := b.Build()
	assert.Equal(t, []schema.TraceRow{{
		TraceID:              h(`{"os":"Android","test":"one"}`),
		Corpus:               "",
		GroupingID:           h(`{"test":"one"}`),
		Keys:                 paramtools.Params{"os": "Android", "test": "one"},
		MatchesAnyIgnoreRule: schema.NBNull,
	}, {
		TraceID:              h(`{"os":"Mac","test":"one"}`),
		Corpus:               "",
		GroupingID:           h(`{"test":"one"}`),
		Keys:                 paramtools.Params{"os": "Mac", "test": "one"},
		MatchesAnyIgnoreRule: schema.NBNull,
	}}, data.Traces)
	assert.Equal(t, []schema.SecondaryBranchValueRow{{
		BranchName:   "gerrit_cl1",
		VersionName:  "gerrit_ps1",
		TraceID:      h(`{"os":"Android","test":"one"}`),
		Digest:       d(t, digestB),
		GroupingID:   h(`{"test":"one"}`),
		OptionsID:    h(`{"opt":"opt"}`),
		SourceFileID: h("tryjob1.txt"),
		TryjobID:     "bb_tryjob1",
	}, {
		BranchName:   "gerrit_cl1",
		VersionName:  "gerrit_ps1",
		TraceID:      h(`{"os":"Mac","test":"one"}`),
		Digest:       d(t, digestC),
		GroupingID:   h(`{"test":"one"}`),
		OptionsID:    h(`{"opt":"opt"}`),
		SourceFileID: h("tryjob2.txt"),
		TryjobID:     "bb_tryjob2",
	}}, data.SecondaryBranchValues)
}

func TestPatchsetBuilder_TriageSameDigest_FinalLabelCorrect(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys("test")
	b.SetDigests(map[rune]types.Digest{'B': digestB})
	b.CommitsWithData().Insert("123", "author_one", "subject_one", "2020-12-05T16:00:00Z")
	b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History("B").Keys([]paramtools.Params{{"test": "one"}}).
		OptionsAll(paramtools.Params{"opt": "opt"}).
		IngestedFrom([]string{"first"}, []string{"2020-12-05T16:30:00Z"})

	cl := b.AddChangelist("cl1", "gerrit", "user1", "whatever", schema.StatusOpen)
	ps := cl.AddPatchset("ps1", "ffff111111111111111111111111111111111111", 1)
	ps.DataWithCommonKeys(paramtools.Params{"os": "Android"}).
		Digests(digestB).Keys([]paramtools.Params{{"test": "one"}}).
		OptionsAll(paramtools.Params{"opt": "opt"}).
		FromTryjob("tryjob1", "bb", "TRYJOB1", "tryjob1.txt", "2020-12-05T16:30:00Z")

	cl.AddTriageEvent("user1", "2020-12-12T09:31:19Z").
		ExpectationsForGrouping(paramtools.Params{"test": "one"}).
		Negative(digestB)
	cl.AddTriageEvent("user1", "2020-12-12T09:31:32Z").
		ExpectationsForGrouping(paramtools.Params{"test": "one"}).
		Triage(digestB, schema.LabelNegative, schema.LabelUntriaged)
	data := b.Build()
	assert.Len(t, data.ExpectationRecords, 2)
	firstTriageRecord := data.ExpectationRecords[0].ExpectationRecordID
	secondTriageRecord := data.ExpectationRecords[1].ExpectationRecordID
	assert.Equal(t, []schema.SecondaryBranchExpectationRow{{
		BranchName:          "gerrit_cl1",
		GroupingID:          h(`{"test":"one"}`),
		Digest:              d(t, digestB),
		Label:               schema.LabelUntriaged, // second triage should be in effect
		ExpectationRecordID: secondTriageRecord,
	}}, data.SecondaryBranchExpectations)
	assert.Equal(t, []schema.ExpectationDeltaRow{{
		ExpectationRecordID: firstTriageRecord,
		GroupingID:          h(`{"test":"one"}`),
		Digest:              d(t, digestB),
		LabelBefore:         schema.LabelUntriaged,
		LabelAfter:          schema.LabelNegative,
	}, {
		ExpectationRecordID: secondTriageRecord,
		GroupingID:          h(`{"test":"one"}`),
		Digest:              d(t, digestB),
		LabelBefore:         schema.LabelNegative,
		LabelAfter:          schema.LabelUntriaged,
	}}, data.ExpectationDeltas)
	assert.Equal(t, []schema.ExpectationRow{{
		GroupingID:          h(`{"test":"one"}`),
		Digest:              d(t, digestB),
		Label:               schema.LabelUntriaged,
		ExpectationRecordID: nil,
	}}, data.Expectations)
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

// The generated gitHash is simply the sha1 sum of the commit id.
func gitHash(cID schema.CommitID) string {
	h := sha1.Sum([]byte(cID))
	return hex.EncodeToString(h[:])
}
