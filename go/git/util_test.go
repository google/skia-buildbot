package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNormURL(t *testing.T) {
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)

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
	unittest.SmallTest(t)

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
	unittest.SmallTest(t)

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
