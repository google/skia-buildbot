package git

import (
	"net/url"
	"strings"
)

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
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", err
	}

	// If the scheme is ssh we have to account for the scp-like syntax with a ':'
	host := parsedURL.Host
	if parsedURL.Scheme == "ssh" {
		host = strings.Replace(host, ":", "/", 1)
	}

	// Trim trailing slashes and the ".git" extension.
	path := strings.TrimRight(strings.TrimSuffix(parsedURL.Path, ".git"), "/")
	path = "/" + strings.TrimLeft(path, "/:")
	return host + path, nil
}
