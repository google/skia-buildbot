package iap

import "strings"

// AllowedFromList controls access by checking an email address
// against a list of approved domain names and email addresses.
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
func (w *AllowedFromList) Member(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	if w.domains[parts[1]] || w.emails[email] {
		return true
	}
	return false
}

// TODO Need AllowedFromFile that checks for file updates.
