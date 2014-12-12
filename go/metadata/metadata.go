package metadata

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"skia.googlesource.com/buildbot.git/go/util"
)

// get retrieves the named value from the Metadata server. See
// https://developers.google.com/compute/docs/metadata
//
// level should be either "instance" or "project" for the kind of
// metadata to retrieve.
func get(name string, level string) (string, error) {
	req, err := http.NewRequest("GET", "http://metadata/computeMetadata/v1/"+level+"/attributes/"+name, nil)
	if err != nil {
		return "", fmt.Errorf("metadata.Get() failed to build request: %s", err)
	}
	c := util.NewTimeoutClient()
	req.Header.Add("Metadata-Flavor", "Google")
	resp, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("metadata.Get() failed to make HTTP request for %s: %s", name, err)
	}
	defer resp.Body.Close()
	value, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to read %s from metadata server: %s", name, err)
	}
	return string(value), nil
}

// Get retrieves the named value from the instance Metadata server. See
// https://developers.google.com/compute/docs/metadata
func Get(name string) (string, error) {
	return get(name, "instance")
}

// ProjectGet retrieves the named value from the project Metadata server. See
// https://developers.google.com/compute/docs/metadata
func ProjectGet(name string) (string, error) {
	return get(name, "project")
}

// MustGet is Get() that panics on error.
func MustGet(keyname string) string {
	value, err := Get(keyname)
	if err != nil {
		glog.Fatalf("Unable to obtain %q from metadata server: %s.", keyname, err)
	}
	return value
}

func Must(s string, err error) string {
	if err != nil {
		glog.Fatalf("Failed to read metadata: %s.", err)
	}
	return s
}
