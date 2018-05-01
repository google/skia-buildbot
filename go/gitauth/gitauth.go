// A Go implementation of https://gerrit.googlesource.com/gcompute-tools/+/master/git-cookie-authdaemon
package gitauth

import (
	"fmt"
	"io"
	"strings"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
)

var (
	// SCOPES are the list of valid scopes for git auth.
	SCOPES = []string{
		"https://www.googleapis.com/auth/gerritcodereview",
		"https://www.googleapis.com/auth/source.full_control",
		"https://www.googleapis.com/auth/source.read_write",
		"https://www.googleapis.com/auth/source.read_only",
	}
)

const (
	REFRESH        = time.Minute
	RETRY_INTERVAL = 5 * time.Second
)

// GitAuth continuously updates the git cookie.
//
type GitAuth struct {
	tokenSource oauth2.TokenSource
	filename    string
}

func (g *GitAuth) update_cookie() (time.Duration, error) {
	token, err := g.tokenSource.Token()
	if err != nil {
		return RETRY_INTERVAL, fmt.Errorf("Failed to retrieve token: %s", err)
	}
	refresh_in := token.Expiry.Sub(time.Now())
	refresh_in -= REFRESH
	if refresh_in < 0 {
		refresh_in = REFRESH
	}
	contents := []string{}
	// As documented on a random website: https://xiix.wordpress.com/2006/03/23/mozillafirefox-cookie-format/
	contents = append(contents, fmt.Sprintf("source.developers.google.com\tFALSE\t/\tTRUE\t%d\to\t%s", token.Expiry.Unix(), token.AccessToken))
	contents = append(contents, fmt.Sprintf(".googlesource.com\tTRUE\t/\tTRUE\t%d\to\t%s", token.Expiry.Unix(), token.AccessToken))
	err = util.WithWriteFile(g.filename, func(w io.Writer) error {
		_, err := w.Write([]byte(strings.Join(contents, "\n")))
		return err
	})
	if err != nil {
		return RETRY_INTERVAL, fmt.Errorf("Failed to write new cookie file: %s", err)
	}
	return refresh_in, nil
}

// New returns a new *GitAuth.
//
// tokenSource - An oauth2.TokenSource authorized to access the repository, with an appropriate scope set.
// filename - The name of the git cookie file, e.g. "~/.git-credential-cache/cookie".
//
// Git must be told about the location of the Cookie file with:
//
//    git config --global http.cookie.file ~/.git-credential-cache/cookie
//
func New(tokenSource oauth2.TokenSource, filename string) (*GitAuth, error) {
	g := &GitAuth{
		tokenSource: tokenSource,
		filename:    filename,
	}
	refresh_in, err := g.update_cookie()
	if err != nil {
		return nil, fmt.Errorf("Failed to get initial git cookie: %s", err)
	}
	go func() {
		time.Sleep(refresh_in)
		refresh_in, err = g.update_cookie()
		if err != nil {
			sklog.Errorf("Failed to get initial git cookie: %s", err)
		}
	}()
	return g, nil
}
