package login

import (
	"strings"

	"go.skia.org/infra/go/allowed"
)

const (
	// defaultAdminList is list of users we consider to be admins as a
	// fallback when we can't retrieve the list from metadata.
	defaultAdminList = "borenet@google.com jcgregorio@google.com kjlubick@google.com lovisolo@google.com rmistry@google.com cmumford@google.com"
)

var (
	// activeUserDomainAllowList is the list of domains that are allowed to
	// log in.
	activeUserDomainAllowList map[string]bool

	// activeUserEmailAllowList is the list of email addresses that are
	// allowed to log in (even if the domain is not explicitly allowed).
	activeUserEmailAllowList map[string]bool

	// activeAdminEmailAllowList is the list of email addresses that are
	// allowed to perform admin tasks.
	activeAdminEmailAllowList map[string]bool

	// Auth groups which determine whether a given user has particular types
	// of access. If nil, fall back on domain and individual email allow lists.
	adminAllow allowed.Allow
	editAllow  allowed.Allow
	viewAllow  allowed.Allow
)

// splitAuthAllowList splits the given allow list into a set of domains and a
// set of individual emails
func splitAuthAllowList(allowList string) (map[string]bool, map[string]bool) {
	domains := map[string]bool{}
	emails := map[string]bool{}

	for _, entry := range strings.Fields(allowList) {
		trimmed := strings.ToLower(strings.TrimSpace(entry))
		if strings.Contains(trimmed, "@") {
			emails[trimmed] = true
		} else {
			domains[trimmed] = true
		}
	}

	return domains, emails
}

// setActiveAllowLists initializes activeUserDomainAllowList and
// activeUserEmailAllowList from authAllowList.
func setActiveAllowLists(authAllowList string) {
	if adminAllow != nil || editAllow != nil || viewAllow != nil {
		return
	}
	activeUserDomainAllowList, activeUserEmailAllowList = splitAuthAllowList(authAllowList)
	_, activeAdminEmailAllowList = splitAuthAllowList(defaultAdminList)
}
