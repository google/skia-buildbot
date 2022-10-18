package urls

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	baseURL       = "https://chromedriver.storage.googleapis.com"
	baseCanaryURL = "https://commondatastorage.googleapis.com/chromium-browser-snapshots"
)

// DownloadURLs returns various URLs for downloading drivers.
type DownloadURLs struct {
	// prefix is the os/architecture prefix used in the Canary URLs. Example:
	// "Mac_Arm".
	prefix string

	// filename of the driver to download, also incorporates the os/arch, for
	// example: "chromedriver_linux64.zip".
	filename string

	// filename in the canary repository.
	canaryFilename string
}

var downloadURLsLookup = map[string]DownloadURLs{
	"darwin/amd64": {prefix: "Mac", filename: "chromedriver_mac64.zip", canaryFilename: "chromedriver_mac64.zip"},
	"darwin/arm64": {prefix: "Mac_Arm", filename: "chromedriver_mac_arm64.zip", canaryFilename: "chromedriver_mac64.zip"},
	"linux/amd64":  {prefix: "Linux_x64", filename: "chromedriver_linux64.zip", canaryFilename: "chromedriver_linux64.zip"},
}

// NewDownloadURLs returns a DownloadURLs for the given os and arch.
//
// Valid values of 'os' and 'arch' come from runtime.GOOS and runtime.GOARCH
// respectively.
func NewDownloadURLs(os, arch string) (DownloadURLs, error) {
	ret, ok := downloadURLsLookup[fmt.Sprintf("%s/%s", os, arch)]
	if !ok {
		return ret, skerr.Fmt("Unavailable combination: %s/%s", os, arch)
	}
	return ret, nil
}

// LatestURL returns a URL that has as it's body the version identifier of the
// latest build of the Stable driver.
func (d DownloadURLs) LatestURL() string {
	return fmt.Sprintf("%s/%s", baseURL, "LATEST_RELEASE")
}

// LatestCanaryURL returns a URL that has as it's body the version identifier of
// the latest build of the Canary driver.
func (d DownloadURLs) LatestCanaryURL() string {
	return fmt.Sprintf("%s/%s/LAST_CHANGE", baseCanaryURL, d.prefix)
}

// DriverURL returns the URL of a zip file that contains the Stable driver for
// the given `version`.
func (d DownloadURLs) DriverURL(version string) string {
	return fmt.Sprintf("%s/%s/%s", baseURL, version, d.filename)
}

// CanaryDriverURL returns the URL of a zip file that contains the Canary driver for
// the given `version`.
func (d DownloadURLs) CanaryDriverURL(version string) string {
	return fmt.Sprintf("%s/%s/%s/%s", baseCanaryURL, d.prefix, version, d.canaryFilename)
}

// GetVersionFromURL returns the whitespace trimmed string in the body of the
// given URL.
func GetVersionFromURL(url string, client *http.Client) (string, error) {
	b, err := GetBodyFromURL(url, client)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// GetBodyFromURL returns the bytes of the body at the given URL.
func GetBodyFromURL(url string, client *http.Client) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Failed to load: %s", resp.Status)
	}
	return ioutil.ReadAll(resp.Body)
}
