package git

import (
	"fmt"
	"net/url"
	"strings"

	"go.skia.org/infra/go/skerr"
)

// LogFromTo returns a string which is used to log from one commit to another.
// It is important to note that:
// - The results will include the second commit but not the first.
// - The results include all commits reachable from the first commit which are
//   not reachable from the second, ie. if there is a merge in the given
//   range, the results will include that line of history and not just the
//   commits which are descendants of the first commit. If you want only commits
//   which are ancestors of the second commit AND descendants of the first, you
//   should use LogLinear.
func LogFromTo(from, to string) string {
	return fmt.Sprintf("%s..%s", from, to)
}

// NormalizeURL strips everything from the URL except for the host and the path.
// A trailing ".git" is also stripped. The purpose is to allow for small
// variations in repo URL to be recognized as the same repo. The URL needs to
// contain a valid transport protocol, e.g. https, ssh.
// These URLs will all return 'github.com/skia-dev/textfiles':
//
//    "https://github.com/skia-dev/textfiles.git"
//    "ssh://git@github.com/skia-dev/textfiles"
//    "ssh://git@github.com:skia-dev/textfiles.git"
//
func NormalizeURL(inputURL string) (string, error) {
	// If the scheme is ssh we have to account for the scp-like syntax with a ':'
	const ssh = "ssh://"
	if strings.HasPrefix(inputURL, ssh) {
		inputURL = ssh + strings.Replace(inputURL[len(ssh):], ":", "/", 1)
	}

	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", skerr.Wrapf(err, "parsing inputURL")
	}

	host := parsedURL.Host
	// Trim trailing slashes and the ".git" extension.
	path := strings.TrimRight(strings.TrimSuffix(parsedURL.Path, ".git"), "/")
	path = "/" + strings.TrimLeft(path, "/:")
	return host + path, nil
}
