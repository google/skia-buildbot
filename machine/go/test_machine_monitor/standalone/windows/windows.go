// Package windows contains Windows-specific pieces of interrogation which are nonetheless testable
// on arbitrary platforms.
package windows

import (
	"regexp"

	"go.skia.org/infra/go/skerr"
)

var platformRegex = regexp.MustCompile(`Microsoft Windows ([^ ]+)`)

// version is built using the format string "%d.%d.%d Build %d", where the last 2 %d
// tokens are filled identically.
var versionRegex = regexp.MustCompile(`\d+\.\d+\.(\d+) `)

func OSVersions(platform, version string) ([]string, error) {
	groups := platformRegex.FindStringSubmatch(platform)
	if groups == nil {
		return nil, skerr.Fmt("couldn't parse Windows platform string %q", platform)
	}
	major := groups[1] // Possible values include 8.1, 10, XP, and Server.

	groups = versionRegex.FindStringSubmatch(version)
	if groups == nil {
		return nil, skerr.Fmt("couldn't parse Windows version string %q", version)
	}
	build := groups[1]

	// We don't detect service pack numbers at the moment, which are a pre-Windows-10 phenomenon. If
	// we wanted to, we could extract them from the end of platform or do our own registry query for
	// CSDVersion.

	return []string{"Windows", "Windows-" + major, "Windows-" + major + "-" + build}, nil
}
