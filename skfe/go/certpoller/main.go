// certpoller polls the current set of nginx SSL certs for this machine in Google Compute
// Project level metadata and updates the local copies if they change.
//
// Presumes that the project metadata keys map to file names in the following way:
//
//  skiamonitor-com-key    ->   /etc/nginx/ssl/skiamonitor_com.key
//
// Run by passing in all the metadata keys on the command line:
//
//    $ certpoller skiamonitor-com-key skiaalerts-com-key
//
package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

// cert contains information about one SSL certificate file.
type cert struct {
	metadata string // The name of the cert in GCE project level metadata.
	file     string // The local filename of the cert.
	etag     string // The etag of the cert when last retrieved from GCE project level metadata.
}

// fileFromMetadata turns a metadata key name into a local filename.
//
// For example:
//    skiamonitor-com-key    ->   /etc/nginx/ssl/skiamonitor_com.key
func fileFromMetadata(metadata string) string {
	return "/etc/nginx/ssl/" + strings.Replace(strings.Replace(metadata, "-", "_", 1), "-", ".", 1)
}

// md5File returns the md5File hash of the given file.
func md5File(filename string) string {
	cmd := exec.Command("sudo", "md5sum", filename)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		// We are OK to Fatal here because md5s will only be checked on startup.
		sklog.Fatalf("Failed to calculate md5sum for %s: %s", filename, err)
	}
	return strings.Split(out.String(), " ")[0]
}

// get retrieves the metadata file if it's changed and writes it to the correct location.
func get(client *http.Client, cert *cert) error {
	// We aren't using the metadata package here because we need to set/get etags.
	r, err := http.NewRequest("GET", "http://metadata/computeMetadata/v1/project/attributes/"+cert.metadata, nil)
	if err != nil {
		return fmt.Errorf("Failed to create request for metadata: %s", err)
	}
	r.Header.Set("Metadata-Flavor", "Google")
	if cert.etag != "" {
		r.Header.Set("If-None-Match", cert.etag)
	}
	resp, err := client.Do(r)
	if err != nil {
		return fmt.Errorf("Failed to retrieve metadata for %s: %s", cert.metadata, err)
	}
	if resp.StatusCode != 200 {
		if cert.etag != "" && resp.StatusCode == 304 {
			// The etag is set and matches what we've already seen, so the file is
			// unchanged. Note that this can't happen the first time get() is called
			// for each file as etag won't be set, so we'll fall through to the code
			// below in that case.
			sklog.Infof("etag unchanged for %s: %s", cert.file, cert.etag)
			return nil
		} else {
			return fmt.Errorf("Unexpected status response: %d: %s", resp.StatusCode, resp.Status)
		}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read metadata for %s: %s", cert.metadata, err)
	}
	sklog.Infof("Read body for %s: len %d", cert.file, len(body))

	// Store the etag to be used for the next time through get().
	cert.etag = resp.Header.Get("ETag")

	newMD5Hash := fmt.Sprintf("%x", md5.Sum(body))
	if cert.etag != "" || newMD5Hash != md5File(cert.file) {
		// Write out the file to a temp file then sudo mv it over to the right location.
		f, err := ioutil.TempFile("", "certpoller")
		if err != nil {
			sklog.Errorf("Failed to create tmp cert file for %s: %s", cert.metadata, err)
		}
		n, err := f.Write(body)
		if err != nil || n != len(body) {
			return fmt.Errorf("Failed to write cert len(body)=%d, n=%d: %s", len(body), n, err)
		}
		tmpName := f.Name()
		if err := f.Close(); err != nil {
			return fmt.Errorf("Failed to close temporary file: %v", err)
		}
		cmd := exec.Command("sudo", "mv", tmpName, cert.file)
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("Failed to mv certfile into place for %s: %s", cert.metadata, err)
		}
	}
	sklog.Infof("Successfully wrote %s", cert.file)
	return nil
}

func main() {
	common.InitWithMust(
		"certpoller",
		common.CloudLoggingOpt(),
	)

	client := httputils.NewTimeoutClient()
	retVal := 255

	// Populate certs based on cmd-line args.
	for _, metadata := range flag.Args() {
		c := &cert{
			metadata: metadata,
			file:     fileFromMetadata(metadata),
			etag:     "",
		}
		err := get(client, c)
		if err != nil {
			sklog.Fatalf("Failed to retrieve the cert %s: %s", c, err)
		}
		retVal = 0
	}

	os.Exit(retVal)
}
