package resolver

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/bazel/gazelle/cpp/common"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestResolveDepsForCImport_RepoFiles_Success(t *testing.T) {
	unittest.SmallTest(t)

	test := func(inputFile, expectedPackage, expectedRule string) {
		t.Run(inputFile, func(t *testing.T) {
			actual := resolveDepsForCImport(inputFile)
			expectedLabel := label.New("", expectedPackage, expectedRule)
			assert.True(t, expectedLabel.Equal(actual), "%#v != %#v", expectedLabel, actual)
		})
	}
	test("include/gpu/GrDirectContext.h", "include/gpu", "GrDirectContext_hdr")
	test("src/gpu/BaseDevice.h", "src/gpu", "BaseDevice_hdr")
	test("src/gpu/gl/builders/GrGLProgramBuilder.h", "src/gpu/gl/builders", "GrGLProgramBuilder_hdr")
	test("src/core/SkUtil.cpp", "src/core", "SkUtil_src")
}

func TestSetDeps_IgnoreDuplicates(t *testing.T) {
	unittest.SmallTest(t)

	testLabel := label.New("@skia", "src/core", "SkFile_hdr")

	test := func(name string, newLabels []label.Label, expectedDeps []string) {
		t.Run(name, func(t *testing.T) {
			r := rule.NewRule(common.GeneratedCCAtomRule, "SkFile_hdr")
			setDeps(r, testLabel, newLabels)
			assert.Equal(t, expectedDeps, r.AttrStrings("deps"))
		})
	}

	test("oneNewDep",
		[]label.Label{label.New("@skia", "include/core", "SkTypes_hdr")},
		[]string{"//include/core:SkTypes_hdr"})
	test("multipleNewDeps",
		[]label.Label{
			label.New("@skia", "include/core", "SkTypes_hdr"),
			label.New("@skia", "include/core", "SkMath_hdr"),
			label.New("@skia", "src/core", "SkMacros_hdr"),
			label.New("@skia", "include/core", "SkMath_hdr"),
			label.New("@skia", "src/gpu", "GrGpu_hdr"),
		},
		[]string{
			"//include/core:SkMath_hdr",
			"//include/core:SkTypes_hdr",
			"//src/gpu:GrGpu_hdr",
			":SkMacros_hdr",
		})
}

func TestThirdPartyDep_Success(t *testing.T) {
	unittest.SmallTest(t)

	assert.Equal(t, label.New("", "third_party", "libpng"), thirdPartyDep("//third_party:libpng"))
}

func TestResolve_RespectsIgnorePrefix(t *testing.T) {
	unittest.SmallTest(t)

	inputRule := rule.NewRule(common.GeneratedCCAtomRule, "MyFile_src")
	inputLabel := label.New("@SomeRepo", "src/core/alpha/beta", "MyFile_src")
	importsToResolve := []string{"include/gamma.h", "src/core/delta.cpp", "png.h", "iota/deprecated_file.h"}
	imports := common.NewImports(importsToResolve, nil, map[string]string{
		"png.h":                  "//third_party:libpng",
		"iota/deprecated_file.h": "SK_GAZELLE_IGNORE Not needed with bazel builds",
	})

	r := CppResolver{}

	r.Resolve(nil, nil, nil, inputRule, imports, inputLabel)

	assert.Equal(t, []string{
		"//include:gamma_hdr",
		"//src/core:delta_src",
		"//third_party:libpng",
	}, inputRule.AttrStrings("deps"))
}

func TestResolve_LooksAtRepoHeadersAndSystemHeadersForThirdParty(t *testing.T) {
	unittest.SmallTest(t)

	inputRule := rule.NewRule(common.GeneratedCCAtomRule, "MyFile_src")
	inputLabel := label.New("@SomeRepo", "src/core/alpha/beta", "MyFile_src")
	repoIncludes := []string{"png.h", "freetype/ftadvanc.h"}
	systemIncludes := []string{"string", "cmath", "jerror.h", "iota/deprecated_file.h"}
	imports := common.NewImports(repoIncludes, systemIncludes, map[string]string{
		"png.h":                  "//third_party:libpng",
		"freetype/ftadvanc.h":    "//third_party:freetype2",
		"jerror.h":               "//third_party:libjpeg_turbo",
		"iota/deprecated_file.h": "SK_GAZELLE_IGNORE Not needed with bazel builds",
	})

	r := CppResolver{}

	r.Resolve(nil, nil, nil, inputRule, imports, inputLabel)

	assert.Equal(t, []string{
		"//third_party:freetype2",
		"//third_party:libjpeg_turbo",
		"//third_party:libpng",
	}, inputRule.AttrStrings("deps"))
}

func TestResolve_AbsolutePathsAllowedInFileMap(t *testing.T) {
	unittest.SmallTest(t)

	inputRule := rule.NewRule(common.GeneratedCCAtomRule, "MyFile_src")
	inputLabel := label.New("@SomeRepo", "src/core/alpha/beta", "MyFile_src")
	repoIncludes := []string{"png.h", "freetype/ftadvanc.h"}
	systemIncludes := []string{"string", "cmath", "jerror.h"}
	imports := common.NewImports(repoIncludes, systemIncludes, map[string]string{
		"png.h":               "@libpng//bazel:settings",
		"freetype/ftadvanc.h": "@freetype2//:freetype2",
		"jerror.h":            "@libjpeg_turbo//:jpeg",
	})

	r := CppResolver{}

	r.Resolve(nil, nil, nil, inputRule, imports, inputLabel)

	assert.Equal(t, []string{
		"@freetype2//:freetype2",
		"@libjpeg_turbo//:jpeg",
		"@libpng//bazel:settings",
	}, inputRule.AttrStrings("deps"))
}
