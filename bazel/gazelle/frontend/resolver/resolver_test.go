package resolver

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver_ImportsIndex_IndexThenFind_Success(t *testing.T) {
	rslv := &Resolver{}

	// Index TypeScript imports provided by a fake //measurements/units:units target.
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

	// Index TypeScript imports provided by a fake //measurements/conversion:conversion target.
	conversion, err := label.Parse("//measurements/conversion:conversion")
	require.NoError(t, err)
	rslv.indexImportsProvidedByRule("ts", []string{"measurements/conversion/conversion"}, "ts_library", conversion)

	// Index Sass imports provided by a fake //shared:styles target.
	styles, err := label.Parse("//shared:styles")
	require.NoError(t, err)
	rslv.indexImportsProvidedByRule("sass", []string{"shared/styles"}, "sass_library", styles)

	// These are only used by findRuleThatProvidesImport to log errors to stdout.
	fromRuleKind := ""
	fromLabel := label.NoLabel

	// Assert that findRuleThatProvidesImport returns the correct rules for the given imports.
	assert.Equal(t, ruleKindAndLabel{"ts_library", units}, rslv.findRuleThatProvidesImport("ts", "measurements/units/customary", fromRuleKind, fromLabel))
	assert.Equal(t, ruleKindAndLabel{"ts_library", units}, rslv.findRuleThatProvidesImport("ts", "measurements/units/imperial", fromRuleKind, fromLabel))
	assert.Equal(t, ruleKindAndLabel{"ts_library", units}, rslv.findRuleThatProvidesImport("ts", "measurements/units/international", fromRuleKind, fromLabel))
	assert.Equal(t, ruleKindAndLabel{"ts_library", conversion}, rslv.findRuleThatProvidesImport("ts", "measurements/conversion/conversion", fromRuleKind, fromLabel))
	assert.Equal(t, ruleKindAndLabel{"sass_library", styles}, rslv.findRuleThatProvidesImport("sass", "shared/styles", fromRuleKind, fromLabel))

	// Assert that findRuleThatProvidesImport correctly handles unknown imports.
	assert.Equal(t, noRuleKindAndLabel, rslv.findRuleThatProvidesImport("ts", "no/such/import", fromRuleKind, fromLabel))
	assert.Equal(t, noRuleKindAndLabel, rslv.findRuleThatProvidesImport("sass", "no/such/import", fromRuleKind, fromLabel))
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
