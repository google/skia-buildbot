package datakitchensink

import (
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/sql/databuilder"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

// Build creates a set of data that covers many common testing scenarios.
func Build() schema.Tables {
	// TODO(kjlubick) replace this placeholder (from databuilder_test) with actual test data.
	b := databuilder.TablesBuilder{}
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

	b.ComputeDiffMetricsFromImages("img", "2020-12-14T14:14:14Z")

	return b.Build()
}

const (
	digestA = types.Digest("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	digestB = types.Digest("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	digestC = types.Digest("cccccccccccccccccccccccccccccccc")
	digestD = types.Digest("dddddddddddddddddddddddddddddddd")
)
