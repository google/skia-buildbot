/*
   Package rotations provides helpers for sheriff/trooper rotations.
*/
package rotations

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sort"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	SkiaGardenerURL  = "https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gardener"
	InfraGardenerURL = "https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-infra-gardener"

	errMsgTmpl = "Unable to parse rotation member(s) from %q. JSON: %q"
)

// FromURL attempts to load the current rotation member(s) from the given URL.
func FromURL(c *http.Client, url string) ([]string, error) {
	// Hit the URL to get the email address. Expect JSON or a JS file which
	// document.writes the email(s) in a comma-separated list.
	resp, err := c.Get(url)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer util.Close(resp.Body)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var rotation struct {
		Emails   []string `json:"emails"`
		Username string   `json:"username"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&rotation); err != nil {
		return nil, skerr.Wrapf(err, errMsgTmpl, url, body)
	}
	found := false
	emails := util.StringSet{}
	if rotation.Emails != nil {
		emails.AddLists(rotation.Emails)
		found = true
	}
	if rotation.Username != "" {
		emails[rotation.Username] = true
		found = true
	}
	if found {
		// Sort for consistency in testing.
		rv := emails.Keys()
		sort.Strings(rv)
		return rv, nil
	} else {
		return nil, skerr.Wrapf(skerr.Fmt("Missing 'emails' and 'username' field"), errMsgTmpl, url, body)
	}
}
