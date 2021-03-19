package resolver

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver_ImportsIndex_IndexThenFind_Success(t *testing.T) {
	rslv := &Resolver{}

	units, err := label.Parse("//measurements/units:units")
	require.NoError(t, err)
	rslv.indexImportsProvidedByRule(
		"ts",
		[]string{
			"measurements/units/customary",
			"measurements/units/imperial",
			"measurements/units/international",
		},
		"ts_library",
		units)

	conversion, err := label.Parse("//measurements/conversion:conversion")
	require.NoError(t, err)
	rslv.indexImportsProvidedByRule("ts", []string{"measurements/conversion/conversion"}, "ts_library", conversion)

	styles, err := label.Parse("//shared:styles")
	require.NoError(t, err)
	rslv.indexImportsProvidedByRule("sass", []string{"shared/styles"}, "sass_library", styles)

	test := func(lang, importPath string, expectedOutput ruleKindAndLabel) {
		// These are only used to log errors to stdout.
		fromRuleKind := ""
		fromLabel := label.NoLabel

		t.Run(lang+", "+importPath, func(t *testing.T) {
			assert.Equal(t, expectedOutput, rslv.findRuleThatProvidesImport(lang, importPath, fromRuleKind, fromLabel))
		})
	}

	test("ts", "measurements/units/customary", ruleKindAndLabel{"ts_library", units})
	test("ts", "measurements/units/imperial", ruleKindAndLabel{"ts_library", units})
	test("ts", "measurements/units/international", ruleKindAndLabel{"ts_library", units})
	test("ts", "measurements/conversion/conversion", ruleKindAndLabel{"ts_library", conversion})
	test("sass", "shared/styles", ruleKindAndLabel{"sass_library", styles})

	test("ts", "no/such/import", noRuleKindAndLabel)
	test("sass", "no/such/import", noRuleKindAndLabel)
}

func TestResolver_ImportsIndex_InvalidLang_Panics(t *testing.T) {
	rslv := &Resolver{}

	assert.Panics(t, func() {
		rslv.indexImportsProvidedByRule("nosuchlang", []string{}, "", label.NoLabel)
	}, "Unknown language: nosuchlang.")

	assert.Panics(t, func() {
		rslv.findRuleThatProvidesImport("nosuchlang", "", "", label.NoLabel)
	}, "Unknown language: nosuchlang.")
}
