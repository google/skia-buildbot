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

func TestIsCommitHash(t *testing.T) {
	test := func(s string, expect bool, name string) {
		require.Equal(t, expect, IsCommitHash(s))
	}
	test("", false, "empty")
	test("abc123", false, "too short")
	test(".*", false, "invalid characters")
	test("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false, "slightly too short")
	test("gggggggggggggggggggggggggggggggggggggggg", false, "g is not valid")
	test("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", false, "capitals not valid")
	test("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", false, "too long")

	test("0000000000000000000000000000000000000000", true, "valid 0")
	test("1111111111111111111111111111111111111111", true, "valid 1")
	test("2222222222222222222222222222222222222222", true, "valid 2")
	test("3333333333333333333333333333333333333333", true, "valid 3")
	test("4444444444444444444444444444444444444444", true, "valid 4")
	test("5555555555555555555555555555555555555555", true, "valid 5")
	test("6666666666666666666666666666666666666666", true, "valid 6")
	test("7777777777777777777777777777777777777777", true, "valid 7")
	test("8888888888888888888888888888888888888888", true, "valid 8")
	test("9999999999999999999999999999999999999999", true, "valid 9")
	test("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true, "valid a")
	test("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", true, "valid b")
	test("cccccccccccccccccccccccccccccccccccccccc", true, "valid c")
	test("dddddddddddddddddddddddddddddddddddddddd", true, "valid d")
	test("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", true, "valid e")
	test("ffffffffffffffffffffffffffffffffffffffff", true, "valid f")
	test("e2e44d8f6febe328c7da13feaec3fc4710b41bae", true, "real hash")
}
