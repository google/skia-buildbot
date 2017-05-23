package metadata

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
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

	// DATABASE_RW_PASSWORD and DATABASE_ROOT_PASSWORD are the MySQL Database passwords for the
	// readwrite and root user respectively.
	DATABASE_RW_PASSWORD   = "database_readwrite"
	DATABASE_ROOT_PASSWORD = "database_root"

	// AUTH_WHITE_LIST is the comma or whitespace separated list of domains
	// and email address that are allowed to log into an app.
	AUTH_WHITE_LIST = "auth_white_list"

	// ADMIN_WHITE_LIST is the comma or whitespace separated list of domains
	// and email address that are allowed to perform admin tasks.
	ADMIN_WHITE_LIST = "admin_white_list"

	// METADATA_URL is the URL template for metadata. The placeholders are
	// for the level ("instance" or "project") and the metadata key.
	METADATA_URL = "http://metadata/computeMetadata/v1/%s/attributes/%s"

	// WEBHOOK_REQUEST_SALT is used to authenticate webhook requests. The value stored in
	// Metadata is base64-encoded.
	// Value created 2015-08-10 with
	// dd if=/dev/random iflag=fullblock bs=64 count=1 | base64 -w 0
	WEBHOOK_REQUEST_SALT = "webhook_request_salt"

	// JWT_SERVICE_ACCOUNT is the JSON formatted service account.
	JWT_SERVICE_ACCOUNT = "jwt_service_account"

	// NSQ_TEST_SERVER refers to a test server in GCE which runs NSQ for testing purposes.
	NSQ_TEST_SERVER = "nsq-test-server"
)

// get retrieves the named value from the Metadata server. See
// https://developers.google.com/compute/docs/metadata
//
// level should be either "instance" or "project" for the kind of
// metadata to retrieve.
func get(name string, level string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf(METADATA_URL, level, name), nil)
	if err != nil {
		return "", fmt.Errorf("metadata.Get() failed to build request: %s", err)
	}
	c := httputils.NewTimeoutClient()
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

// GetWithDefault is Get, but returns the default value on error.
func GetWithDefault(name, defaultValue string) string {
	if ret, err := Get(name); err == nil {
		return ret
	} else {
		sklog.Warningf("Unable to obtain %q from metadata server: %v", name, err)
		return defaultValue
	}
}

// ProjectGet retrieves the named value from the project Metadata server. See
// https://developers.google.com/compute/docs/metadata
func ProjectGet(name string) (string, error) {
	return get(name, "project")
}

// ProjectGetWithDefault is ProjectGet, but returns the default value on error.
func ProjectGetWithDefault(name, defaultValue string) string {
	if ret, err := ProjectGet(name); err == nil {
		return ret
	} else {
		sklog.Warningf("Unable to obtain %q from metadata server: %v", name, err)
		return defaultValue
	}
}

// MustGet is Get() that panics on error.
func MustGet(keyname string) string {
	value, err := Get(keyname)
	if err != nil {
		sklog.Fatalf("Unable to obtain %q from metadata server: %s.", keyname, err)
	}
	return value
}

func Must(s string, err error) string {
	if err != nil {
		sklog.Fatalf("Failed to read metadata: %s.", err)
	}
	return s
}

// NSQDTestServerAddr returns the address of a test NSQD server used for testing. If
// not running in GCE, this is the local machine.
func NSQDTestServerAddr() string {
	server := ProjectGetWithDefault(NSQ_TEST_SERVER, "127.0.0.1")
	sklog.Errorf("Got test NSQ server: %s", server)
	return fmt.Sprintf("%s:4150", server)
}
