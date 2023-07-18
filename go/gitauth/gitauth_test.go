package gitauth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/exec/testutils"
	"go.skia.org/infra/go/git/git_common"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2"
)

type testTokenSource struct {
	token *oauth2.Token
}

func newTestToken() *testTokenSource {
	return &testTokenSource{
		token: &oauth2.Token{
			AccessToken: "foo",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(10 * time.Minute),
		},
	}
}

func (t *testTokenSource) Token() (*oauth2.Token, error) {
	return t.token, nil
}

func TestNew_UsesTokenSource_WritesCookie(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "cookie")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, err := New(ctx, newTestToken(), filename, false, "")
	require.NoError(t, err)
	assert.Equal(t, filename, g.filename)
	b, err := os.ReadFile(filename)
	require.NoError(t, err)
	lines := strings.Split(string(b), "\n")
	assert.True(t, strings.HasPrefix(lines[0], "source.developers.google.com\tFALSE\t/\tTRUE\t"))
	assert.True(t, strings.HasSuffix(lines[0], "o\tfoo"))
	assert.True(t, strings.HasPrefix(lines[1], ".googlesource.com\tTRUE\t/\tTRUE\t"))
	assert.True(t, strings.HasSuffix(lines[1], "o\tfoo"))

	assert.Equal(t, filename, os.Getenv("GIT_COOKIES_PATH"))

	// Change AccessToken and confirm of makes it into the cookie file.
	g.tokenSource.(*testTokenSource).token.AccessToken = "bar"
	d, err := g.updateCookie(ctx)
	assert.True(t, d.Minutes() < 11)
	require.NoError(t, err)
	b, err = os.ReadFile(filename)
	require.NoError(t, err)
	lines = strings.Split(string(b), "\n")
	assert.True(t, strings.HasPrefix(lines[0], "source.developers.google.com\tFALSE\t/\tTRUE\t"))
	assert.True(t, strings.HasSuffix(lines[0], "o\tbar"))
	assert.True(t, strings.HasPrefix(lines[1], ".googlesource.com\tTRUE\t/\tTRUE\t"))
	assert.True(t, strings.HasSuffix(lines[1], "o\tbar"))
}

func TestNew_UsesConfig_CallsGitAndWritesCookie(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "cookie")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const fakeGitPath = "/path/to/fake/git"
	const gitEmail = "test_user@example.com"
	const expectedTestGitUser = "test_user"

	commandSpy := exec.CommandCollector{}
	gitFinder := func() (string, error) {
		return fakeGitPath, nil
	}
	ctx = git_common.WithGitFinder(ctx, gitFinder)
	ctx = exec.NewContext(ctx, func(ctx context.Context, cmd *exec.Command) error {
		err := commandSpy.Run(ctx, cmd)
		if err != nil {
			return skerr.Wrap(err)
		}
		// Make calls to git --version have non-empty stdout/error so git_common.Version
		// works correctly (which is called on any FindGit command)
		if len(cmd.Args) == 1 && cmd.Args[0] == "--version" {
			_, _ = cmd.CombinedOutput.Write([]byte("git version 2.718.28"))
		}
		return nil
	})

	g, err := New(ctx, newTestToken(), filename, true, gitEmail)
	require.NoError(t, err)
	assert.Equal(t, filename, g.filename)

	executedCommands := commandSpy.Commands()
	testutils.AssertCommandsMatch(t, [][]string{
		{fakeGitPath, "--version"},
		{fakeGitPath, "config", "--global", "http.cookiefile", filename},
		{fakeGitPath, "config", "--global", "user.email", gitEmail},
		{fakeGitPath, "config", "--global", "user.name", expectedTestGitUser},
		{fakeGitPath, "config", "--list", "--show-origin"},
	}, executedCommands)

	b, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Contains(t, string(b), "source.developers.google.com\tFALSE\t/\tTRUE\t")
	assert.Equal(t, filename, os.Getenv("GIT_COOKIES_PATH"))
}
