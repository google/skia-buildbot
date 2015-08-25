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

// Required header for requests to a webhook authenticated using AuthenticateRequest. The value must
// be set to the result of ComputeAuthHashBase64.
const REQUEST_AUTH_HASH_HEADER = "X-Webhook-Auth-Hash"

var requestSalt []byte = nil

// InitRequestSaltForTesting sets requestSalt to "notverysecret". Should be called once at startup
// when running in test mode.
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

// InitRequestSaltFromMetadata reads requestSalt from project metadata and returns any error
// encountered. Should be called once at startup.
func InitRequestSaltFromMetadata() error {
	saltBase64, err := metadata.ProjectGet(metadata.WEBHOOK_REQUEST_SALT)
	if err != nil {
		return err
	}
	if err := setRequestSaltFromBase64([]byte(saltBase64)); err != nil {
		return fmt.Errorf("Could not decode salt from %s: %s", metadata.WEBHOOK_REQUEST_SALT, err)
	}
	return nil
}

// MustInitRequestSaltFromMetadata reads requestSalt from project metadata. Exits the program on
// error. Should be called once at startup.
func MustInitRequestSaltFromMetadata() {
	if err := InitRequestSaltFromMetadata(); err != nil {
		glog.Fatal(err)
	}
}

// InitRequestSaltFromFile reads requestSalt from the given file and returns any error encountered.
// Should be called once at startup.
func InitRequestSaltFromFile(filename string) error {
	saltBase64Bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Could not read the webhook request salt file: %s", err)
	}
	if err = setRequestSaltFromBase64(saltBase64Bytes); err != nil {
		return fmt.Errorf("Could not decode salt from %s: %s", filename, err)
	}
	return nil
}

// MustInitRequestSaltFromFile reads requestSalt from the given file. Exits the program on error.
// Should be called once at startup.
func MustInitRequestSaltFromFile(filename string) {
	if err := InitRequestSaltFromFile(filename); err != nil {
		glog.Fatal(err)
	}
}

// Computes the value for REQUEST_AUTH_HASH_HEADER from the request body. Returns error if
// requestSalt has not been initialized. The result is the base64-encoded SHA-512 hash of the
// request body with requestSalt appended.
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
//  - If the request could not be authenticated as a webhook request, returns the contents of r.Body
//    and an error.
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
