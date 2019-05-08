package roller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Update the current sheriff list.
func getSheriff(parentName, childName, metricsName string, sheriffSources, backupSheriffs []string) ([]string, error) {
	tags := map[string]string{
		"roller": metricsName,
	}
	m := metrics2.GetInt64Metric("autoroll_get_sheriff_success", tags)
	allEmails := []string{}
	for _, s := range sheriffSources {
		emails, err := getSheriffHelper(s)
		if err != nil {
			sklog.Errorf("Failed to retrieve sheriff(s): %s", err)
			m.Update(0)
			if len(backupSheriffs) == 0 {
				return nil, fmt.Errorf("Failed to retrieve sheriffs and no backup sheriffs supplied!  %s", err)
			}
			return backupSheriffs, nil
		}
		allEmails = append(allEmails, emails...)
	}
	m.Update(1)
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
func getSheriffHelper(sheriffConfig string) ([]string, error) {
	// If the passed-in sheriffConfig doesn't look like a URL, it's probably an
	// email address. Use it directly.
	if _, err := url.ParseRequestURI(sheriffConfig); err != nil {
		if strings.Count(sheriffConfig, "@") == 1 {
			return []string{sheriffConfig}, nil
		} else {
			return nil, fmt.Errorf("Sheriff must be an email address or a valid URL; %q doesn't look like either.", sheriffConfig)
		}
	}

	// Hit the URL to get the email address. Expect JSON or a JS file which
	// document.writes the Sheriff(s) in a comma-separated list.
	client := httputils.NewTimeoutClient()
	resp, err := client.Get(sheriffConfig)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	if strings.HasSuffix(sheriffConfig, ".js") {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return getSheriffJS(string(body)), nil
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var sheriff struct {
			Emails   []string `json:"emails"`
			Username string   `json:"username"`
		}
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&sheriff); err != nil {
			return nil, err
		}
		if sheriff.Emails != nil && len(sheriff.Emails) > 0 {
			return sheriff.Emails, nil
		}
		if sheriff.Username != "" {
			return []string{sheriff.Username}, nil
		}
		return nil, fmt.Errorf("Unable to parse sheriff email(s) from %q. JSON: %q", sheriffConfig, body)
	}
}
