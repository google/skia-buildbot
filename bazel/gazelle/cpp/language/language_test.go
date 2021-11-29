package language

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/bazel/gazelle/cpp/common"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestMakeRulesFromFiles_UsesSuffixToSetAttributes(t *testing.T) {
	unittest.SmallTest(t)

	fileNames := []string{"alpha.h", "alpha.cpp", "beta.H", "beta.cc", "gamma.hh", "delta.cxx",
		"gamma.c", "delta.hpp", "epsilon.h", "zeta.cpp", "SkTypes.h", "SkUtils.cpp"}

	rules := makeRulesFromFiles(fileNames)

	assertRulesMatch(t, []*rule.Rule{
		headerAtom("alpha_hdr", "alpha.h"),
		sourceAtom("alpha_src", "alpha.cpp"),
		headerAtom("beta_hdr", "beta.H"),
		sourceAtom("beta_src", "beta.cc"),
		headerAtom("gamma_hdr", "gamma.hh"),
		sourceAtom("gamma_src", "gamma.c"),
		headerAtom("delta_hdr", "delta.hpp"),
		sourceAtom("delta_src", "delta.cxx"),
		headerAtom("epsilon_hdr", "epsilon.h"),
		sourceAtom("zeta_src", "zeta.cpp"),
		headerAtom("SkTypes_hdr", "SkTypes.h"),
		sourceAtom("SkUtils_src", "SkUtils.cpp"),
	}, rules)
}

func TestFindEmptyRules_Success(t *testing.T) {
	unittest.SmallTest(t)

	rulesInExistingBuildFile := []*rule.Rule{
		headerAtom("alpha_hdr", "alpha.h"),
		sourceAtom("alpha_src", "alpha.cpp"),
		headerAtom("SkTypes_hdr", "SkTypes.h"),
		sourceAtom("SkUtils_src", "SkUtils.cpp"),
	}

	rulesFromSourceFiles := []*rule.Rule{
		headerAtom("Alpha_hdr", "Alpha.h"),
		sourceAtom("Alpha_src", "Alpha.cpp"),
		headerAtom("SkTypes_hdr", "SkTypes.h"),
	}

	actualEmpty := findEmptyRules(rulesInExistingBuildFile, rulesFromSourceFiles)

	// The returned rules just have the kind and name set
	assertRulesMatch(t, []*rule.Rule{
		rule.NewRule(common.GeneratedCCAtomRule, "alpha_hdr"),
		rule.NewRule(common.GeneratedCCAtomRule, "alpha_src"),
		rule.NewRule(common.GeneratedCCAtomRule, "SkUtils_src"),
	}, actualEmpty)
}

func headerAtom(name, header string) *rule.Rule {
	r := rule.NewRule(common.GeneratedCCAtomRule, name)
	r.SetAttr("visibility", []string{"//:__subpackages__"})
	r.SetAttr("hdrs", []string{header})
	return r
}

func sourceAtom(name, source string) *rule.Rule {
	r := rule.NewRule(common.GeneratedCCAtomRule, name)
	r.SetAttr("visibility", []string{"//:__subpackages__"})
	r.SetAttr("srcs", []string{source})
	return r
}

type ruleAttrs struct {
	name string
	hdrs []string
	srcs []string
}

func assertRulesMatch(t *testing.T, expected, actual []*rule.Rule) {
	var eRules []ruleAttrs
	for _, r := range expected {
		eRules = append(eRules, ruleAttrs{
			name: r.Name(),
			hdrs: r.AttrStrings("hdrs"),
			srcs: r.AttrStrings("srcs"),
		})
	}
	var aRules []ruleAttrs
	for _, r := range actual {
		aRules = append(aRules, ruleAttrs{
			name: r.Name(),
			hdrs: r.AttrStrings("hdrs"),
			srcs: r.AttrStrings("srcs"),
		})
	}
	// This assertion is easier to debug when the two variables do not match.
	assert.ElementsMatch(t, eRules, aRules)
	assert.ElementsMatch(t, expected, actual)
}
