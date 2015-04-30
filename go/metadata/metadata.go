package metadata

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/util"
)

// GCE project level metadata keys.
const (
	// APIKEY is used for access to APIs that don't need OAuth 2.0.
	APIKEY = "apikey"

	// COOKIESALT, CLIENT_ID, and CLIENT_SECRET are used for login.
	COOKIESALT    = "cookiesalt"
	CLIENT_ID     = "client_id"
	CLIENT_SECRET = "client_secret"

	// GMAIL_CACHED_TOKEN, GMAIL_CLIENT_ID, and GMAIL_CLIENT_SECRET are used for sending mail
	// from alerts@skia.org.
	GMAIL_CACHED_TOKEN  = "gmail_cached_token"
	GMAIL_CLIENT_ID     = "gmail_clientid"
	GMAIL_CLIENT_SECRET = "gmail_clientsecret"

	// INFLUXDB_NAME and INFLUXDB_PASSWORD are used for accessing InfluxDB.
	INFLUXDB_NAME     = "influxdb_name"
	INFLUXDB_PASSWORD = "influxdb_password"

	// DATABASE_RW_PASSWORD and DATABASE_ROOT_PASSWORD are the MySQL Database passwords for the
	// readwrite and root user respectively.
	DATABASE_RW_PASSWORD   = "database_readwrite"
	DATABASE_ROOT_PASSWORD = "database_root"

	// AUTH_WHITE_LIST is the comma or whitespace separated list of domains
	// and email address that are allowed to log into an app.
	AUTH_WHITE_LIST = "auth_white_list"
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
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP response has status %d", resp.StatusCode)
	}
	defer util.Close(resp.Body)
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
