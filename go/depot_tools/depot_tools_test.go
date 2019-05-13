package depot_tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	// Example DEPS file where the revision is in a variable.
	TEST_VAR_DEPS = `
	vars = {
		'skia_git': 'https://skia.googlesource.com',
		# Three lines of non-changing comments so that
		# the commit queue can handle CLs rolling Skia
		# and whatever else without interference from each other.
		'skia_revision': 'f3b4e16c36a6c789fc129aa3bd15c34b44ee8743',
		# Three lines of non-changing comments so that
		# the commit queue can handle CLs rolling Skia
		# and whatever else without interference from each other.
	}

	deps = {
		'../skia': Var('skia_git') + '/skia.git' + '@' +  Var('skia_revision'),
	}`

	// Example DEPS file where the revision is in a URL.
	TEST_URL_DEPS = `
	use_relative_paths = True

	deps = {
			'skia/':
			'https://chromium.googlesource.com/skia/@5cf7b6175ecf2c469bc6fedb815ba68f748f02d2'
	}

	recursedeps = [ "skia/" ]
	`

	// Commits bembedded in the DEPS files
	VAR_COMMIT = "f3b4e16c36a6c789fc129aa3bd15c34b44ee8743"
	URL_COMMIT = "5cf7b6175ecf2c469bc6fedb815ba68f748f02d2"
)

func TestDEPSExtractor(t *testing.T) {
	unittest.SmallTest(t)

	ext_1 := NewRegExDEPSExtractor(DEPSSkiaVarRegEx)
	ret, err := ext_1.ExtractCommit(TEST_VAR_DEPS, nil)
	assert.NoError(t, err)
	assert.Equal(t, VAR_COMMIT, ret)

	ext_2 := NewRegExDEPSExtractor(DEPSSkiaURLRegEx)
	ret, err = ext_2.ExtractCommit(TEST_URL_DEPS, nil)
	assert.NoError(t, err)
	assert.Equal(t, URL_COMMIT, ret)
}
