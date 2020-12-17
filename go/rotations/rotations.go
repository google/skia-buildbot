/*
   Package rotations provides helpers for sheriff/trooper rotations.
*/
package rotations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"go.skia.org/infra/go/util"
)

const (
	SkiaGardenerURL  = "https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gardener"
	InfraGardenerURL = "https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-infra-gardener"
)

// FromURL attempts to load the current sheriffs/troopers from the given URL.
func FromURL(c *http.Client, url string) ([]string, error) {
	// Hit the URL to get the email address. Expect JSON or a JS file which
	// document.writes the email(s) in a comma-separated list.
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	if strings.HasSuffix(url, ".js") {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return getRotationJS(string(body)), nil
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var rotation struct {
			Emails   []string `json:"emails"`
			Username string   `json:"username"`
		}
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&rotation); err != nil {
			return nil, err
		}
		if rotation.Emails != nil && len(rotation.Emails) > 0 {
			return rotation.Emails, nil
		}
		if rotation.Username != "" {
			return []string{rotation.Username}, nil
		}
		return nil, fmt.Errorf("Unable to parse sheriff/trooper email(s) from %q. JSON: %q", url, body)
	}
}

// Parse the rotation from JS. Expects the list in this format:
// document.write('somebody, somebodyelse')
// TODO(borenet): Remove this once Chromium has a proper sheriff endpoint, ie.
// https://bugs.chromium.org/p/chromium/issues/detail?id=769804
func getRotationJS(js string) []string {
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
