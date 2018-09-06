package allowed

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"sync"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	fsnotify "gopkg.in/fsnotify.v1"
)

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

	if a.domains[parts[1]] || a.emails[email] {
		return true
	}
	return false
}

func (a *AllowedFromList) Emails() []string {
	ret := make([]string, 0, len(a.emails))
	for k, _ := range a.emails {
		ret = append(ret, k)
	}
	return ret
}

// Googlers creates a new AllowedFromList which restricts to only users logged
// in with an @google.com account.
func Googlers() *AllowedFromList {
	return NewAllowedFromList([]string{"google.com"})
}

// AllowedFromFile implements Allow by reading the list of emails and domains
// from a file. The file is watched for changes and re-read when they occur.
// The file format is one email address or domain name per line.
//
// It implements Allow.
type AllowedFromFile struct {
	filename string
	allowed  *AllowedFromList
	mutex    sync.RWMutex
}

// NewAllowedFromFile creates a new *AllowedFromFile from the given filename.
//
// Example:
//
//	 emails := `fred@example.com
// barney@example.com
// chromium.org`
//   ioutil.WriteFile("/etc/my_app/auth", []byte(emails), 0644)
//   a := NewAllowedFromFile("/etc/my_app/auth")
//
// The presumption is that an AllowedFromFile will be created
// at startup and if creation fails then the application will
// not start.kk
func NewAllowedFromFile(filename string) (*AllowedFromFile, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("Failed to create watcher: %s", err)
	}
	a := &AllowedFromFile{
		filename: filename,
	}
	if err := a.reload(); err != nil {
		util.Close(watcher)
		return nil, fmt.Errorf("Failed to initially load allowed from file %q: %s", filename, err)
	}
	go func() {
		for {
			select {
			case <-watcher.Events:
				if err := a.reload(); err != nil {
					sklog.Errorf("Failed to reload allowed file %q: %s", filename, err)
				}
			case err := <-watcher.Errors:
				sklog.Errorf("Watcher error %q: %s", filename, err)
			}
		}
	}()
	if err := watcher.Add(filename); err != nil {
		util.Close(watcher)
		return nil, fmt.Errorf("Failed to watch Allowed file %q: %s", filename, err)
	}
	return a, nil
}

func (a *AllowedFromFile) reload() error {
	b, err := ioutil.ReadFile(a.filename)
	if err != nil {
		return err
	}
	newAllowed := NewAllowedFromList(strings.Split(string(b), "\n"))

	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.allowed = newAllowed

	return nil
}

func (a *AllowedFromFile) Member(email string) bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return a.allowed.Member(email)
}

func (a *AllowedFromFile) Emails() []string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return a.allowed.Emails()
}

// Union is an Allow which includes members of multiple other Allows.
type Union []Allow

func UnionOf(allows ...Allow) Allow {
	return Union(allows)
}

func (allows Union) Member(email string) bool {
	for _, a := range allows {
		if a.Member(email) {
			return true
		}
	}
	return false
}

func (allows Union) Emails() []string {
	emails := util.StringSet{}
	for _, a := range allows {
		emails.AddLists(a.Emails())
	}
	rv := emails.Keys()
	sort.Strings(rv)
	return rv
}
