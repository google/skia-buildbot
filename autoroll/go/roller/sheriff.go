package roller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
)

// Update the current sheriff list.
func getSheriff(parentName, childName string, sheriffSources []string) ([]string, error) {
	allEmails := []string{}
	for _, s := range sheriffSources {
		emails, err := getSheriffHelper(s)
		if err != nil {
			return nil, err
		}
		// TODO(borenet): Do we need this any more?
		if strings.Contains(parentName, "Chromium") && childName != "WebRTC" {
			for i, s := range emails {
				emails[i] = strings.Replace(s, "google.com", "chromium.org", 1)
			}
		}
		allEmails = append(allEmails, emails...)
	}
	return allEmails, nil
}

// Parse the sheriff list from JS. Expects the list in this format:
// document.write('somebody, somebodyelse')
// TODO(borenet): Remove this once Chromium has a proper sheriff endpoint, ie.
// https://bugs.chromium.org/p/chromium/issues/detail?id=769804
func getSheriffJS(js string) []string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(js), "document.write('"), "')")
	list := strings.Split(trimmed, ",")
	rv := make([]string, 0, len(list))
	for _, name := range list {
		name = strings.TrimSpace(name)
		if name != "" {
			rv = append(rv, name+"@chromium.org")
		}
	}
	return rv
}

// Helper for loading the sheriff list.
func getSheriffHelper(sheriff string) ([]string, error) {
	// If the passed-in sheriff doesn't look like a URL, it's probably an
	// email address. Use it directly.
	if _, err := url.ParseRequestURI(sheriff); err != nil {
		if strings.Count(sheriff, "@") == 1 {
			return []string{sheriff}, nil
		} else {
			return nil, fmt.Errorf("Sheriff must be an email address or a valid URL; %q doesn't look like either.", sheriff)
		}
	}

	// Hit the URL to get the email address. Expect JSON or a JS file which
	// document.writes the Sheriff(s) in a comma-separated list.
	client := httputils.NewTimeoutClient()
	resp, err := client.Get(sheriff)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	if strings.HasSuffix(sheriff, ".js") {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return getSheriffJS(string(body)), nil
	} else {
		var sheriff struct {
			Emails   []string `json:"emails"`
			Username string   `json:"username"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&sheriff); err != nil {
			return nil, err
		}
		if sheriff.Emails != nil && len(sheriff.Emails) > 0 {
			return sheriff.Emails, nil
		}
		if sheriff.Username != "" {
			return []string{sheriff.Username}, nil
		}
		return nil, fmt.Errorf("Unable to parse sheriff email(s) from JSON.")
	}
}
