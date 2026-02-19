package api

import (
	"regexp"
)

// getOverrideNonProdHost removes the specified suffixes from the host string if they are followed by .*.goog or .*.app.
// This is to ensure that requests from different non-prod environments (autopush, lts, qa, staging) are routed to the main environment.
func getOverrideNonProdHost(host string) string {
	re := regexp.MustCompile(`(-autopush|-lts|-qa|-staging)(\.corp\.goog|\.luci\.app)$`)
	return re.ReplaceAllString(host, "$2")
}
