// Utility methods for implementing authenticated webhooks.
package webhook

import (
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/metadata"
	skutil "go.skia.org/infra/go/util"
)

// Required header for requests to CTFE V2. The value must be the base64-encoded SHA-512 hash of
// the request body with requestSalt appended.
const REQUEST_AUTH_HASH_HEADER = "X-Webhook-Auth-Hash"

var requestSalt []byte = nil

// Call once at startup when running in test mode.
func InitRequestSaltForTesting() {
	requestSalt = []byte("notverysecret")
}

func setRequestSaltFromBase64(saltBase64 []byte) error {
	enc := base64.StdEncoding
	decodedLen := enc.DecodedLen(len(saltBase64))
	salt := make([]byte, decodedLen)
	n, err := enc.Decode(salt, saltBase64)
	if err != nil {
		return err
	}
	requestSalt = salt[:n]
	return nil
}

// Call once at startup to read requestSalt from project metadata.
func MustInitRequestSaltFromMetadata() {
	saltBase64 := metadata.Must(metadata.ProjectGet(metadata.WEBHOOK_REQUEST_SALT))
	if err := setRequestSaltFromBase64([]byte(saltBase64)); err != nil {
		glog.Fatalf("Could not decode salt from %s: %v", metadata.WEBHOOK_REQUEST_SALT, err)
	}
}

// Call once at startup to read requestSalt from the given file.
func MustInitRequestSaltFromFile(filename string) {
	saltBase64Bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		glog.Fatalf("Could not read the webhook request salt file: %s", err)
	}
	if err = setRequestSaltFromBase64(saltBase64Bytes); err != nil {
		glog.Fatalf("Could not decode salt from %s: %v", filename, err)
	}
}

// Computes the value for REQUEST_AUTH_HASH_HEADER from the request body. Returns error if
// requestSalt has not been initialized.
func ComputeAuthHashBase64(data []byte) (string, error) {
	if len(requestSalt) == 0 {
		return "", fmt.Errorf("requestSalt is uninitialized.")
	}
	data = append(data, requestSalt...)
	hash := sha512.Sum512(data)
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// Authenticates a webhook request.
//  - If an error occurs reading r.Body, returns nil and the error.
//  - If the request could not be authenticated as a webhook request, returns the contents of r.Body and an error.
//  - Otherwise, returns the contents of r.Body and nil.
// In all cases, closes r.Body.
func AuthenticateRequest(r *http.Request) ([]byte, error) {
	defer skutil.Close(r.Body)
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	headerHashBase64 := r.Header.Get(REQUEST_AUTH_HASH_HEADER)
	if headerHashBase64 == "" {
		return data, fmt.Errorf("No authentication header %s", REQUEST_AUTH_HASH_HEADER)
	}
	dataHashBase64, err := ComputeAuthHashBase64(data)
	if err != nil {
		return data, err
	}
	if headerHashBase64 == dataHashBase64 {
		return data, nil
	}
	return data, fmt.Errorf("Authentication header %s: %s did not match.", REQUEST_AUTH_HASH_HEADER, headerHashBase64)
}
