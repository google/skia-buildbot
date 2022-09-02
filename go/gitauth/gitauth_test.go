package gitauth

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/util"
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

func TestNew(t *testing.T) {
	dir, err := ioutil.TempDir("", "gitauth")
	filename := filepath.Join(dir, "cookie")
	assert.NoError(t, err)
	defer util.RemoveAll(dir)
	g, err := New(newTestToken(), filename, false, "")
	assert.NoError(t, err)
	assert.Equal(t, filename, g.filename)
	b, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)
	lines := strings.Split(string(b), "\n")
	assert.True(t, strings.HasPrefix(lines[0], "source.developers.google.com\tFALSE\t/\tTRUE\t"))
	assert.True(t, strings.HasSuffix(lines[0], "o\tfoo"))
	assert.True(t, strings.HasPrefix(lines[1], ".googlesource.com\tTRUE\t/\tTRUE\t"))
	assert.True(t, strings.HasSuffix(lines[1], "o\tfoo"))

	// Change AccessToken and confirm of makes it into the cookie file.
	g.tokenSource.(*testTokenSource).token.AccessToken = "bar"
	d, err := g.updateCookie()
	assert.True(t, d.Minutes() < 11)
	assert.NoError(t, err)
	b, err = ioutil.ReadFile(filename)
	assert.NoError(t, err)
	lines = strings.Split(string(b), "\n")
	assert.True(t, strings.HasPrefix(lines[0], "source.developers.google.com\tFALSE\t/\tTRUE\t"))
	assert.True(t, strings.HasSuffix(lines[0], "o\tbar"))
	assert.True(t, strings.HasPrefix(lines[1], ".googlesource.com\tTRUE\t/\tTRUE\t"))
	assert.True(t, strings.HasSuffix(lines[1], "o\tbar"))
}
