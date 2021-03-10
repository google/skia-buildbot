package frontend

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestFindSkElementAndDemoPageSrcs_Success(t *testing.T) {
	unittest.SmallTest(t)

	files := []string{
		"index.ts",
		"my-element-sk.ts",
		"my-element-sk.scss",
		"my-element-sk_test.ts",
		"my-element-sk-demo.html",
		"my-element-sk-demo.ts",
		"my-element-sk-demo.scss",
		"my-element-sk_puppeteer_test.ts",
		"more_styles.scss",
		"test_data.ts",
		"demo_data.ts",
		"util.ts",
		"util_test.ts",
	}

	test := func(name, skElementName string, expectedElementsSrcs skElementSrcs, expectedDemoPageSrcs skPageSrcs) {
		t.Run(name, func(t *testing.T) {
			elementSrcs, demoPageSrcs := findSkElementAndDemoPageSrcs(skElementName, files)
			assert.Equal(t, expectedElementsSrcs, elementSrcs)
			assert.Equal(t, expectedDemoPageSrcs, demoPageSrcs)
		})
	}

	test(
		"sources match the given element name, returns the correct sources",
		"my-element-sk",
		skElementSrcs{
			indexTs: "index.ts",
			ts:      "my-element-sk.ts",
			scss:    "my-element-sk.scss",
		},
		skPageSrcs{
			html: "my-element-sk-demo.html",
			ts:   "my-element-sk-demo.ts",
			scss: "my-element-sk-demo.scss",
		})

	test(
		"sources do not match the given element name, empty response",
		"another-element-sk",
		skElementSrcs{},
		skPageSrcs{},
	)

	test(
		"empty element name, empty response",
		"",
		skElementSrcs{},
		skPageSrcs{},
	)
}
