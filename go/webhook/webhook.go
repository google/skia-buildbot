// Utility methods for implementing authenticated webhooks.
//
// All requests must either be over a private channel (e.g. https) or must be
// idempotent and return no data. Requests sent via an open channel (e.g. http)
// could be resent by an attacker.
package webhook

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

// Required header for requests to a webhook authenticated using AuthenticateRequest. The value must
// be set to the result of ComputeAuthHashBase64.
const REQUEST_AUTH_HASH_HEADER = "X-Webhook-Auth-Hash"

var requestSalt []byte = nil

// InitRequestSaltForTesting sets requestSalt to "notverysecret". Should be called once at startup
// when running in test mode.
//
// To test a webhook endpoint using curl, the following commands should work:
// $ DATA='my post request'
// $ AUTH="$(echo -n "${DATA}notverysecret" | sha512sum | xxd -r -p - | base64 -w 0)"
// $ curl -v -H "X-Webhook-Auth-Hash: $AUTH" -d "$DATA" http://localhost:8000/endpoint
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

// InitRequestSaltFromMetadata reads requestSalt from the specified project metadata
// and returns any error encountered. Should be called once at startup.
func InitRequestSaltFromMetadata(metadataKey string) error {
	saltBase64, err := metadata.ProjectGet(metadataKey)
	if err != nil {
		return err
	}
	if err := setRequestSaltFromBase64([]byte(saltBase64)); err != nil {
		return fmt.Errorf("Could not decode salt from %s: %s", metadataKey, err)
	}
	return nil
}

// MustInitRequestSaltFromMetadata reads requestSalt from the specified project
// metadata. Exits the program on error. Should be called once at startup.
func MustInitRequestSaltFromMetadata(metadataKey string) {
	if err := InitRequestSaltFromMetadata(metadataKey); err != nil {
		sklog.Fatal(err)
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
		sklog.Fatal(err)
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

// NewRequest is similar to http.NewRequest, but adds the REQUEST_AUTH_HASH_HEADER for
// authentication.
func NewRequest(method, urlStr string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hash, err := ComputeAuthHashBase64(body)
	if err != nil {
		return nil, err
	}
	req.Header.Set(REQUEST_AUTH_HASH_HEADER, hash)
	return req, nil
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
