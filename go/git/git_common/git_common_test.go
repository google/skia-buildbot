package git_common

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestFindGit(t *testing.T) {
	unittest.SmallTest(t)

	execCount := 0
	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		execCount++
		return exec.DefaultRun(ctx, cmd)
	})
	ctx := exec.NewContext(context.Background(), mockRun.Run)

	check := func() {
		git, major, minor, err := FindGit(ctx)
		require.NoError(t, err)
		require.NotEqual(t, "", git)
		require.NotEqual(t, "git", git)
		require.NotEqual(t, 0, major)
		require.NotEqual(t, 0, minor)
		// TODO(borenet): We want to ensure that we get Git from CIPD
		// on all bots and servers, but we don't want to impose that
		// restriction on developers.
		//require.True(t, IsFromCIPD(git))
	}
	check()
	require.Equal(t, 1, execCount)

	// Ensure that we cached the results.
	check()
	require.Equal(t, 1, execCount)
}

func TestValidateRef(t *testing.T) {
	unittest.SmallTest(t)

	v := func(ref, expectErr string) {
		err := ValidateRef(ref)
		if expectErr == "" {
			assert.NoError(t, err)
		} else {
			assert.NotNil(t, err)
			if err != nil {
				assert.True(t, strings.Contains(err.Error(), expectErr), err.Error())
			}
		}
	}

	// Empty.
	v("", "Must contain at least one \"/\"")

	// Commit hashes.
	v("abcdef", "Must contain at least one \"/\"")
	v("abcdefa", "")
	v("abcde12345abcde12345abcde", "")
	v("abcde12345abcde12345abcde12345abcde12345", "")
	v("abcde12345abcde12345abcde12345abcde123456", "Must contain at least one \"/\"")

	// 1. No component can begin with "." or end with ".lock"
	v(".bad/ref", "No component can begin with \".\"")
	v("bad/.ref", "No component can begin with \".\"")
	v("no/.lock", "No component can begin with \".\" or end with \".lock\"")

	// 2. Must contain at least one "/"
	v("badref", "Must contain at least one \"/\"")

	// 3. Cannot contain ".."
	v("not/ok..", "Cannot contain \"..\"")

	// 4. Cannot contain ASCII control characters, space, tilde, caret, or colon.
	tmpl4 := "not/%s/allowed"
	msg4 := "Cannot contain ASCII control characters, space, tilde, caret, or colon"
	for i := 0; i < 32; i++ {
		v(fmt.Sprintf(tmpl4, string(rune(i))), msg4)
	}
	v(fmt.Sprintf(tmpl4, string(rune(127))), msg4)
	for _, ch := range []string{" ", "~", "^", ":"} {
		v(fmt.Sprintf(tmpl4, ch), msg4)
	}

	// 5. Cannot contain question mark, asterisk, or open bracket.
	v("not?/allowed", "Cannot contain question mark, asterisk, or open bracket")
	v("*not*/allowed", "Cannot contain question mark, asterisk, or open bracket")
	v("not/[allowed]", "Cannot contain question mark, asterisk, or open bracket")

	// 6. Cannot begin or end with a slash or contain multiple consecutive slashes.
	v("/no/please", "Cannot begin or end with a slash or contain multiple consecutive slashes")
	v("this/either/", "Cannot begin or end with a slash or contain multiple consecutive slashes")
	v("definitely//not", "Cannot begin or end with a slash or contain multiple consecutive slashes")

	// 7. Cannot end with a dot.
	v("emphatic/no.", "Cannot end with a dot")

	// 8. Cannot contain "@{".
	v("why/would/you@{", "Cannot contain \"@{\"")

	// 9. Cannot be "@".
	// This case is handled by rule #2...
	v("@", "Must contain at least one \"/\"") // v("@", "Cannot be \"@\"")

	// 10. Cannot contain \.
	v("no/\\escape", "Cannot contain \"\\\"")

	// These are allowed.
	v("refs/heads/master", "")
	v("refs/heads/\"quoted\"", "")
	v("refs/whoa!", "")
	v("refs/@me", "")
	v("#1/be$t/(_ref`+;}", "")
}
