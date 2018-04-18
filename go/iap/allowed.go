package iap

import (
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"go.skia.org/infra/go/sklog"
	fsnotify "gopkg.in/fsnotify.v1"
)

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

// AllowedFromFile implements Allow by reading the list of emails and domains
// from a file. The file is watched for changes and re-read when they occur.
// The file format is one email address or domain name per line.
//
// It implements Allow.
type AllowedFromFile struct {
	filename string
	allowed  *AllowedFromList
	mutex    sync.Mutex
}

// NewAllowedFromList creates a new *AllowedFromFile from the given filename.
//
// Example:
//
//	 emails := `fred@example.com
// barney@example.com
// chromium.org`
//   ioutil.WriteFile("/etc/my_app/auth", []byte(emails), 0644)
//   a := NewAllowedFromFile("/etc/my_app/auth")
//
func NewAllowedFromFile(filename string) (*AllowedFromFile, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("Failed to create watcher: %s", err)
	}
	w := &AllowedFromFile{
		filename: filename,
	}
	if err := w.reload(); err != nil {
		return nil, fmt.Errorf("Failed to initially load allowed from file %q: %s", filename, err)
	}
	go func() {
		for {
			select {
			case <-watcher.Events:
				if err := w.reload(); err != nil {
					sklog.Errorf("Failed to reload allowed file %q: %s", filename, err)
				}
			case err := <-watcher.Errors:
				sklog.Errorf("Watcher error:", err)
			}
		}
	}()
	if err := watcher.Add(filename); err != nil {
		return nil, fmt.Errorf("Failed to watch Allowed file %q: %s", filename, err)
	}
	return w, nil
}

func (w *AllowedFromFile) reload() error {
	b, err := ioutil.ReadFile(w.filename)
	if err != nil {
		return err
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.allowed = NewAllowedFromList(strings.Split(string(b), "\n"))
	return nil
}

func (w *AllowedFromFile) Member(email string) bool {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	return w.allowed.Member(email)
}
