package gerrit

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Files returns all the files in the given Gerrit 'change', aka issue.
//
// If 'revision', aka patch, is the empty string then the most recent patch is
// used.
func Files(c *http.Client, change, revision string) ([]string, error) {
	if revision == "" {
		revision = "current"
	}

	// Query by hitting a URL of the form:
	// https://skia-review.googlesource.com/changes/81121/revisions/current/files/
	url := fmt.Sprintf("https://skia-review.googlesource.com/changes/%s/revisions/%s/files/", change, revision)
	sklog.Infof("Sending request to: %q", url)
	prefix := make([]byte, 5)
	resp, err := c.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Failed to get commit file list from Gerrit: %s", err)
	}
	defer util.Close(resp.Body)
	// Trim off the first 5 chars, which is Gerrit's anti-XSS protection.
	_, err = resp.Body.Read(prefix)
	// The Gerrit response is a map of filenames to extra info, we only need the filenames.
	files := map[string]interface{}{}
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("Failed to get decode file list from Gerrit: %s", err)
	}
	ret := []string{}
	for filename, _ := range files {
		ret = append(ret, filename)
	}
	return ret, nil
}
