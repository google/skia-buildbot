package frontend

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestFileFilter_Filter_Success(t *testing.T) {
	unittest.SmallTest(t)

	ff := fileFilter{
		keep: []*regexp.Regexp{
			regexp.MustCompile(`.+\.ts$`),
			regexp.MustCompile(`.+\.scss$`),
		},
		skip: []*regexp.Regexp{
			regexp.MustCompile(`.+-demo\.ts$`),
			regexp.MustCompile(`.+-demo\.scss$`),
			regexp.MustCompile(`.+_test\.ts$`),
			regexp.MustCompile(`.+_po\.ts$`),
			regexp.MustCompile(`.+\.d\.ts$`),
		},
	}

	files := []string{
		"myapp/modules/foo-sk/index.ts",
		"myapp/modules/foo-sk/foo-sk.ts",
		"myapp/modules/foo-sk/foo-sk.scss",
		"myapp/modules/foo-sk/foo-sk-demo.ts",
		"myapp/modules/foo-sk/foo-sk-demo.scss",
		"myapp/modules/foo-sk/foo-sk-demo.html",
		"myapp/modules/foo-sk/foo-sk_po.ts",
		"myapp/modules/foo-sk/foo-sk_test.ts",
		"myapp/modules/foo-sk/foo-sk_puppeteer_test.ts",
		"util/strings.js",
		"util/strings.d.ts",
		"util/strings_test.ts",
	}

	expected := []string{
		"myapp/modules/foo-sk/index.ts",
		"myapp/modules/foo-sk/foo-sk.ts",
		"myapp/modules/foo-sk/foo-sk.scss",
	}

	require.ElementsMatch(t, expected, ff.filter(files))
}
