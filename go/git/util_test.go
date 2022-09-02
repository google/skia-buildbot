package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
)

func TestNormURL(t *testing.T) {
	httpURL := "https://github.com/skia-dev/textfiles.git"
	normHTTP, err := NormalizeURL(httpURL)
	require.NoError(t, err)
	assert.Equal(t, "github.com/skia-dev/textfiles", normHTTP)

	gitURL := "ssh://git@github.com/skia-dev/textfiles"
	normGit, err := NormalizeURL(gitURL)
	require.NoError(t, err)
	assert.Equal(t, "github.com/skia-dev/textfiles", normGit)

	gitURLWithExt := "ssh://git@github.com:skia-dev/textfiles.git"
	normGitWithExt, err := NormalizeURL(gitURLWithExt)
	require.NoError(t, err)
	assert.Equal(t, "github.com/skia-dev/textfiles", normGitWithExt)
}

func TestSplitTrailers(t *testing.T) {

	test := func(commitMsg string, expectBody, expectTrailers []string) {
		actualBody, actualTrailers := SplitTrailers(commitMsg)
		require.Equal(t, expectBody, actualBody)
		require.Equal(t, expectTrailers, actualTrailers)
	}

	test("", []string{}, []string{})
	test("Hello World", []string{"Hello World"}, []string{})
	test(`Hello World
	`, []string{"Hello World"}, []string{})
	test(`Hello World

Paragraph 2
`, []string{"Hello World", "", "Paragraph 2"}, []string{})
	test(`Hello World

Trailer-Key: trailer-value`, []string{"Hello World", ""}, []string{"Trailer-Key: trailer-value"})
	test(`Hello World

Trailer-Key: trailer-value
`, []string{"Hello World", ""}, []string{"Trailer-Key: trailer-value"})

	test(`Hello World

Paragraph 2

Trailer-Key: trailer-value
T2: V2
Bug: 1234, chromium:5678
`,
		[]string{"Hello World", "", "Paragraph 2", ""},
		[]string{"Trailer-Key: trailer-value", "T2: V2", "Bug: 1234, chromium:5678"})

	test(`Trailer-Key: trailer-value`, []string{}, []string{"Trailer-Key: trailer-value"})
}

func TestJoinTrailers(t *testing.T) {

	test := func(body, trailers []string, expect string) {
		actual := JoinTrailers(body, trailers)
		require.Equal(t, expect, actual)
	}

	test([]string{}, []string{}, "")
	test([]string{"Hello World"}, []string{}, "Hello World\n")
	test([]string{"Hello World", ""}, []string{}, "Hello World\n")
	test(
		[]string{"Hello World", "", "Paragraph 2", ""},
		[]string{},
		`Hello World

Paragraph 2
`)
	test([]string{}, []string{"Trailer-Key: trailer-value"}, "Trailer-Key: trailer-value")

	test(
		[]string{"Hello World", "", "Paragraph 2", ""},
		[]string{"Trailer-Key: trailer-value"},
		`Hello World

Paragraph 2

Trailer-Key: trailer-value`)
}

func TestAddTrailer(t *testing.T) {

	test := func(commitMsg, trailer, expect, expectErr string) {
		actual, err := AddTrailer(commitMsg, trailer)
		require.Equal(t, expect, actual)
		if expectErr != "" {
			require.NotNil(t, err)
			require.Contains(t, err.Error(), expectErr)
		}
	}

	test("", "", "", "\"\" is not a valid git trailer")
	test("", "bogustrailer", "", "\"bogustrailer\" is not a valid git trailer")
	test("", "Trailer-Key: trailer-value", "Trailer-Key: trailer-value", "")
	test("Hello World", "Trailer-Key: trailer-value", `Hello World

Trailer-Key: trailer-value`, "")

	test(`Hello World

Paragraph 2
`,
		"Trailer-Key: trailer-value",
		`Hello World

Paragraph 2

Trailer-Key: trailer-value`, "")

	test(`Hello World

Paragraph 2

K1: V1
`,
		"Trailer-Key: trailer-value",
		`Hello World

Paragraph 2

K1: V1
Trailer-Key: trailer-value`, "")
}

func TestGetFootersMap(t *testing.T) {

	tests := []struct {
		commitMsg      string
		expectedOutput map[string]string
	}{
		{
			commitMsg:      "Test test test\n\nfooter: value",
			expectedOutput: map[string]string{"footer": "value"},
		},
		{
			commitMsg:      "Test test test\n\nfooter-no-space:value",
			expectedOutput: map[string]string{"footer-no-space": "value"},
		},
		{
			commitMsg:      "Test test test\nfake-footer: value",
			expectedOutput: map[string]string{},
		},
		{
			commitMsg:      "Test test test\nfake-footer: value\n\nfooter1: value1\nfooter2: value2",
			expectedOutput: map[string]string{"footer1": "value1", "footer2": "value2"},
		},
	}

	for _, test := range tests {
		require.True(t, deepequal.DeepEqual(test.expectedOutput, GetFootersMap(test.commitMsg)))
	}
}

func TestGetBoolFooterVal(t *testing.T) {

	testFooterName := "test-footer-name"
	tests := []struct {
		footersMap     map[string]string
		footer         string
		expectedOutput bool
	}{
		{
			footersMap: map[string]string{
				testFooterName: "true",
			},
			footer:         testFooterName,
			expectedOutput: true,
		},
		{
			footersMap: map[string]string{
				testFooterName: "false",
			},
			footer:         testFooterName,
			expectedOutput: false,
		},
		{
			footersMap: map[string]string{
				"some-other-footer": "true",
			},
			footer:         testFooterName,
			expectedOutput: false,
		},
		{
			footersMap: map[string]string{
				"some-other-footer": "true",
				testFooterName:      "true",
			},
			footer:         testFooterName,
			expectedOutput: true,
		},
		{
			footersMap:     map[string]string{},
			footer:         testFooterName,
			expectedOutput: false,
		},
		{
			footersMap:     nil,
			footer:         testFooterName,
			expectedOutput: false,
		},
		{
			footersMap: map[string]string{
				testFooterName: "not-a-bool-val",
			},
			footer:         testFooterName,
			expectedOutput: false,
		},
	}

	for _, test := range tests {
		require.Equal(t, test.expectedOutput, GetBoolFooterVal(test.footersMap, test.footer, 1))
	}
}

func TestGetStringFooterVal(t *testing.T) {

	testFooterName := "test-footer-name"
	tests := []struct {
		footersMap     map[string]string
		footer         string
		expectedOutput string
	}{
		{
			footersMap: map[string]string{
				testFooterName: "value",
			},
			footer:         testFooterName,
			expectedOutput: "value",
		},
		{
			footersMap: map[string]string{
				"some-other-footer": "value",
			},
			footer:         testFooterName,
			expectedOutput: "",
		},
		{
			footersMap: map[string]string{
				"some-other-footer": "value1",
				testFooterName:      "value2",
			},
			footer:         testFooterName,
			expectedOutput: "value2",
		},
		{
			footersMap:     map[string]string{},
			footer:         testFooterName,
			expectedOutput: "",
		},
		{
			footersMap:     nil,
			footer:         testFooterName,
			expectedOutput: "",
		},
	}

	for _, test := range tests {
		require.Equal(t, test.expectedOutput, GetStringFooterVal(test.footersMap, test.footer))
	}
}
