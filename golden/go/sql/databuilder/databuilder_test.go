package databuilder

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"

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

func TestBuild_CalledWithValidInput_ProducesCorrectData(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.Commits().
		Append("author_one", "subject_one", "2020-12-05T16:00:00Z").
		Append("author_two", "subject_two", "2020-12-06T17:00:00Z").
		Append("author_three", "subject_three", "2020-12-07T18:00:00Z").
		Append("author_four", "subject_four", "2020-12-08T19:00:00Z")
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

	dir, err := testutils.TestDataDir()
	require.NoError(t, err)
	b.ComputeDiffMetricsFromImages(dir, "2020-12-14T14:14:14Z")

	tables := b.Build()
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
		MatchesAnyIgnoreRule: schema.NBFalse,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:                 `{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`,
		MatchesAnyIgnoreRule: schema.NBFalse,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:                 `{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`,
		MatchesAnyIgnoreRule: schema.NBTrue,
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
		TraceID:       h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		StartCommitID: 0,
		Digest:        d(t, digestA),
	}, {
		TraceID:       h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		StartCommitID: 0,
		Digest:        d(t, digestB),
	}, {
		TraceID:       h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		StartCommitID: 0,
		Digest:        d(t, digestD),
	}, {
		TraceID:       h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		StartCommitID: 0,
		Digest:        d(t, digestC),
	}, {
		TraceID:       h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		StartCommitID: 0,
		Digest:        d(t, digestD),
	}}, tables.TiledTraceDigests)
	assert.ElementsMatch(t, []schema.PrimaryBranchParamRow{
		{Key: "name", Value: "test_one", StartCommitID: 0},
		{Key: "name", Value: "test_two", StartCommitID: 0},
		{Key: "device", Value: "Crosshatch", StartCommitID: 0},
		{Key: "device", Value: "NUC1234", StartCommitID: 0},
		{Key: "os", Value: "Android", StartCommitID: 0},
		{Key: "os", Value: "Windows10.7", StartCommitID: 0},
		{Key: "color_mode", Value: "rgb", StartCommitID: 0},
		{Key: "source_type", Value: "corpus_one", StartCommitID: 0},
		{Key: "ext", Value: "png", StartCommitID: 0},
	}, tables.PrimaryBranchParams)
	assert.ElementsMatch(t, []schema.ValueAtHeadRow{{
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		MostRecentCommitID:   4,
		Digest:               d(t, digestB),
		OptionsID:            pngOptionsID,
		GroupingID:           testOneGroupingID,
		Corpus:               "corpus_one",
		Keys:                 `{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`,
		Label:                schema.LabelUntriaged,
		ExpectationRecordID:  nil,
		MatchesAnyIgnoreRule: schema.NBFalse,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		MostRecentCommitID:   4,
		Digest:               d(t, digestD),
		OptionsID:            pngOptionsID,
		GroupingID:           testTwoGroupingID,
		Corpus:               "corpus_one",
		Keys:                 `{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`,
		Label:                schema.LabelPositive,
		ExpectationRecordID:  &recordIDTwo,
		MatchesAnyIgnoreRule: schema.NBFalse,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`),
		MostRecentCommitID:   3,
		Digest:               d(t, digestD),
		OptionsID:            pngOptionsID,
		GroupingID:           testTwoGroupingID,
		Corpus:               "corpus_one",
		Keys:                 `{"color_mode":"rgb","device":"NUC1234","name":"test_two","os":"Windows10.7","source_type":"corpus_one"}`,
		Label:                schema.LabelPositive,
		ExpectationRecordID:  &recordIDTwo,
		MatchesAnyIgnoreRule: schema.NBTrue,
	}}, tables.ValuesAtHead)
	assert.Equal(t, []schema.IgnoreRuleRow{{
		IgnoreRuleID: firstIgnoreRuleID,
		CreatorEmail: "ignore_author_one",
		UpdatedEmail: "ignore_author_two",
		Expires:      time.Date(2021, time.March, 14, 15, 9, 27, 0, time.UTC),
		Note:         "note 1",
		Query:        `{"does not":["apply","to any trace"]}`,
	}, {
		IgnoreRuleID: secondIgnoreRuleID,
		CreatorEmail: "ignore_author_two",
		UpdatedEmail: "ignore_author_one",
		Expires:      time.Date(2021, time.June, 28, 03, 18, 53, 0, time.UTC),
		Note:         "note 2",
		Query:        `{"device":["NUC1234"],"os":["Windows10.7","Windows10.8"]}`,
	}}, tables.IgnoreRules)
}

func TestBuild_CalledWithChangelistData_ProducesCorrectData(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.Commits().
		Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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

	dir, err := testutils.TestDataDir()
	require.NoError(t, err)
	b.ComputeDiffMetricsFromImages(dir, "2020-12-14T14:14:14Z")

	tables := b.Build()
	assert.Equal(t, []schema.OptionsRow{{
		OptionsID: h(`{"ext":"png"}`),
		Keys:      `{"ext":"png"}`,
	}, {
		OptionsID: h(`{"ext":"png","matcher":"fuzzy","threshold":"2"}`),
		Keys:      `{"ext":"png","matcher":"fuzzy","threshold":"2"}`,
	}}, tables.Options)
	assert.Equal(t, []schema.GroupingRow{{
		GroupingID: h(`{"name":"test_one","source_type":"corpus_one"}`),
		Keys:       `{"name":"test_one","source_type":"corpus_one"}`,
	}, {
		GroupingID: h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:       `{"name":"test_two","source_type":"corpus_one"}`,
	}, {
		GroupingID: h(`{"name":"test_three","source_type":"corpus_one"}`),
		Keys:       `{"name":"test_three","source_type":"corpus_one"}`,
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
		Keys:                 `{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`,
		MatchesAnyIgnoreRule: schema.NBFalse,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_two","source_type":"corpus_one"}`),
		Keys:                 `{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`,
		MatchesAnyIgnoreRule: schema.NBTrue,
	}, {
		TraceID:              h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_three","os":"Android","source_type":"corpus_one"}`),
		Corpus:               "corpus_one",
		GroupingID:           h(`{"name":"test_three","source_type":"corpus_one"}`),
		Keys:                 `{"color_mode":"rgb","device":"Crosshatch","name":"test_three","os":"Android","source_type":"corpus_one"}`,
		MatchesAnyIgnoreRule: schema.NBFalse,
	}}, tables.Traces)
	assert.Equal(t, []schema.TraceValueRow{{
		Shard:        0x3,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_one","os":"Android","source_type":"corpus_one"}`),
		CommitID:     1,
		Digest:       d(t, digestA),
		GroupingID:   h(`{"name":"test_one","source_type":"corpus_one"}`),
		OptionsID:    h(`{"ext":"png"}`),
		SourceFileID: h("crosshatch_file1"),
	}, {
		Shard:        0x4,
		TraceID:      h(`{"color_mode":"rgb","device":"Crosshatch","name":"test_two","os":"Android","source_type":"corpus_one"}`),
		CommitID:     1,
		Digest:       d(t, digestD),
		GroupingID:   h(`{"name":"test_two","source_type":"corpus_one"}`),
		OptionsID:    h(`{"ext":"png"}`),
		SourceFileID: h("crosshatch_file1"),
	}}, tables.TraceValues)
	assert.ElementsMatch(t, []schema.PrimaryBranchParamRow{
		{Key: "name", Value: "test_one", StartCommitID: 0},
		{Key: "name", Value: "test_two", StartCommitID: 0},
		{Key: "device", Value: "Crosshatch", StartCommitID: 0},
		{Key: "os", Value: "Android", StartCommitID: 0},
		{Key: "color_mode", Value: "rgb", StartCommitID: 0},
		{Key: "source_type", Value: "corpus_one", StartCommitID: 0},
		{Key: "ext", Value: "png", StartCommitID: 0},
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
		ExpectationRecordID: &clRecordID,
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
	b.Commits()
	assert.Panics(t, func() {
		b.Commits()
	})
}

func TestCommits_InvalidTime_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	assert.Panics(t, func() {
		b.Commits().Append("fine", "dandy", "no good")
	})
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits() // oops, no commits
	assert.Panics(t, func() {
		b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	})
}

func TestHistory_CalledMultipleTimes_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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

func TestIngestedFrom_CalledWithoutHistory_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	tb := b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"})
	tb.History("A")
	assert.Panics(t, func() {
		tb.IngestedFrom([]string{"file1"}, []string{"not valid date"})
	})
}

func TestGenerateStructs_IncompleteData_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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

func TestGenerateStructs_IdenticalTracesFromTwoSets_Panics(t *testing.T) {
	unittest.SmallTest(t)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	testDir, err := testutils.TestDataDir()
	require.NoError(t, err)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	assert.Panics(t, func() {
		b.ComputeDiffMetricsFromImages(testDir, "2020-12-05T16:00:00Z")
	})
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	assert.Panics(t, func() {
		b.ComputeDiffMetricsFromImages(testDir, "2020-12-05T16:00:00Z")
	})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
	b.AddTracesWithCommonKeys(paramtools.Params{"os": "Android"}).
		History("A")
	// We should have the right data now.
	assert.NotPanics(t, func() {
		b.ComputeDiffMetricsFromImages(testDir, "2020-12-05T16:00:00Z")
	})
}

func TestComputeDiffMetricsFromImages_InvalidTime_Panics(t *testing.T) {
	unittest.SmallTest(t)
	testDir, err := testutils.TestDataDir()
	require.NoError(t, err)

	b := TablesBuilder{}
	b.SetGroupingKeys(types.CorpusField)
	b.SetDigests(map[rune]types.Digest{'A': digestA})
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
	b.Commits().Append("author_one", "subject_one", "2020-12-05T16:00:00Z")
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
