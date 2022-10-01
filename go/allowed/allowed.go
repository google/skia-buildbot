package allowed

import (
	"sort"
	"strings"

	"go.skia.org/infra/go/util"
)

// AnyDomain is the value to use if any domain is allowed.
const AnyDomain = "*"

// Allow is used to enforce additional restrictions on who has access to a site,
// eg. members of a group.
type Allow interface {
	// Member returns true if the given email address has access.
	Member(email string) bool
	Emails() []string
}

// AllowedFromList controls access by checking an email address
// against a list of approved domain names and email addresses.
//
// It implements Allow.
type AllowedFromList struct {
	domains map[string]bool
	emails  map[string]bool
}

// NewAllowedFromList creates a new *AllowedFromList from the list of domain names
// and email addresses.
//
// Example:
//   a := NewAllowedFromList([]string{"google.com", "chromium.org", "someone@example.org"})
//
func NewAllowedFromList(emailsAndDomains []string) *AllowedFromList {
	domains := map[string]bool{}
	emails := map[string]bool{}

	for _, entry := range emailsAndDomains {
		trimmed := strings.ToLower(strings.TrimSpace(entry))
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "@") {
			emails[trimmed] = true
		} else {
			domains[trimmed] = true
		}
	}

	return &AllowedFromList{
		domains: domains,
		emails:  emails,
	}
}

// Member returns true if the given email address is AllowedFromList.
func (a *AllowedFromList) Member(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if parts[1] == "" {
		return false
	}

	if a.domains[AnyDomain] {
		return true
	}

	if a.domains[parts[1]] || a.emails[email] {
		return true
	}
	return false
}

func (a *AllowedFromList) Emails() []string {
	ret := make([]string, 0, len(a.emails))
	for k := range a.emails {
		ret = append(ret, k)
	}
	return ret
}

// Googlers creates a new AllowedFromList which restricts to only users logged
// in with an @google.com account.
func Googlers() *AllowedFromList {
	return NewAllowedFromList([]string{"google.com"})
}

// Union is an Allow which includes members of multiple other Allows.
type Union []Allow

// UnionOf combines multiple Allows together in an "or" fashion.
func UnionOf(allows ...Allow) Allow {
	return Union(allows)
}

// Member returns true if email is a member of any of the Allow in this union.
func (allows Union) Member(email string) bool {
	for _, a := range allows {
		if a.Member(email) {
			return true
		}
	}
	return false
}

// Emails returns a slice of unique emails from the Union.
func (allows Union) Emails() []string {
	emails := util.StringSet{}
	for _, a := range allows {
		emails.AddLists(a.Emails())
	}
	rv := emails.Keys()
	sort.Strings(rv)
	return rv
}
