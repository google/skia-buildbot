package gerrit

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
)

func Files(c *http.Client, change, revision string) ([]string, error) {
	if revision == "" {
		revision = "current"
	}

	// https://skia-review.googlesource.com/changes/81121/revisions/current/files/
	url := fmt.Sprintf("https://skia-review.googlesource.com/changes/%s/revisions/%s/files/", change, revision)
	glog.Infof("Sending request to: %q", url)
	prefix := make([]byte, 5)
	resp, err := c.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Failed to get commit file list from Gerrit: %s", err)
	}
	defer util.Close(resp.Body)
	// Trim off the first 5 chars.
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
