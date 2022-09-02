package webhook

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	expect "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/util"
)

const (
	TEST_SALT        = "notverysecret"
	TEST_SALT_BASE64 = "bm90dmVyeXNlY3JldA=="
	INVALID_BASE64   = "bm90dmVyeXNlY3JldA!!"
)

func TestSetRequestSaltFromBase64Success(t *testing.T) {
	expect.Nil(t, setRequestSaltFromBase64([]byte(TEST_SALT_BASE64)))
	expect.Equal(t, []byte(TEST_SALT), requestSalt)
}

func TestSetRequestSaltFromBase64Corrupt(t *testing.T) {
	err := setRequestSaltFromBase64([]byte(INVALID_BASE64))
	require.Error(t, err)
	expect.Contains(t, err.Error(), "illegal base64 data")
}

func TestMustInitRequestSaltFromFileSuccess(t *testing.T) {
	f, err := ioutil.TempFile("", "webhook_test_salt")
	require.NoError(t, err)
	defer util.Remove(f.Name())
	_, err = f.WriteString(TEST_SALT_BASE64)
	require.NoError(t, err)
	MustInitRequestSaltFromFile(f.Name())
	expect.Equal(t, []byte(TEST_SALT), requestSalt)
}

func TestComputeAuthHashBase64Success(t *testing.T) {
	require.NoError(t, setRequestSaltFromBase64([]byte(TEST_SALT_BASE64)))
	test := func(input, expected string) {
		actual, err := ComputeAuthHashBase64([]byte(input))
		expect.NoError(t, err)
		expect.Equal(t, expected, actual, "Auth hash of %#v with salt %#v", input, TEST_SALT)
	}
	// Expected result obtained via:
	// $ echo -n '<input>notverysecret' | sha512sum | xxd -r -p | base64
	test("", "F+ZnkgguWrbUsJheE3l8fGPacy+Ugf1H9m5y2EyWWC70LihVcDkY7d+Yyx/AQaZdpvHK/oAkFPdzjTe7PuSI6w==")
	test(`{"id": 20}`, "gYIoDyh7VUbHbTQ8SRsnLeOQbqSs+mzlyxEanfqAs9yN6IVYdwvUBrU4rpTtisUoxJg5zU/jVNmp3AJGl6m1fA==")
	test(`\`, "ThSNhyrg99Ms/lxQvgfDVI3X+wMUtNDdRH2xrz48oR3rrjnoHkYI/6bFu4Du4x3QPX0Cz+1pEqvmpWz5G3rtDQ==")
	// From rmistry on skiabot hangout.
	test("QA Engineer walks into a bar. Orders a beer. Orders 0 beers. Orders 999999999 beers. Orders a lizard. Orders -1 beers. Orders a sfdeljknesv.",
		"mLGGNv+vJV8TBGyU+j+tRPifccuB50fDIi+TxIPPeP1au59Y3ngkQwsLb1cXkDFB26TbES1dixVhtolxY5vobA==")
}

func TestComputeAuthHashBase64NotInitialized(t *testing.T) {
	requestSalt = nil
	_, err := ComputeAuthHashBase64([]byte("foo"))
	require.Error(t, err)
	expect.Contains(t, err.Error(), "requestSalt is uninitialized")
}

func TestAuthenticateRequestSuccess(t *testing.T) {
	require.NoError(t, setRequestSaltFromBase64([]byte(TEST_SALT_BASE64)))
	test := func(bodyStr string) {
		body := []byte(bodyStr)
		req, err := http.NewRequest("POST", "http://invalid.", bytes.NewReader(body))
		require.NoError(t, err)
		hash, err := ComputeAuthHashBase64(body)
		require.NoError(t, err)
		req.Header.Set(REQUEST_AUTH_HASH_HEADER, hash)
		actual, err := AuthenticateRequest(req)
		expect.NoError(t, err)
		expect.Equal(t, body, actual)
	}
	test("")
	test("foo")
	postData := url.Values{}
	postData.Set("key", "17")
	postData.Add("foo", "bar")
	postData.Add("foo", "baz")
	test(postData.Encode())
}

func TestAuthenticateRequestNoHeader(t *testing.T) {
	require.NoError(t, setRequestSaltFromBase64([]byte(TEST_SALT_BASE64)))
	body := []byte("my data")
	req, err := http.NewRequest("POST", "http://invalid.", bytes.NewReader(body))
	require.NoError(t, err)
	actual, err := AuthenticateRequest(req)
	require.Error(t, err)
	expect.Contains(t, err.Error(), "No authentication header")
	// Still returns body even though there was an authentication error.
	expect.Equal(t, body, actual)
}

func TestAuthenticateRequestErrorComputingHash(t *testing.T) {
	requestSalt = nil
	body := []byte("my data")
	req, err := http.NewRequest("POST", "http://invalid.", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set(REQUEST_AUTH_HASH_HEADER, "unused")
	actual, err := AuthenticateRequest(req)
	require.Error(t, err)
	expect.Contains(t, err.Error(), "requestSalt is uninitialized")
	// Still returns body even though there was an authentication error.
	expect.Equal(t, body, actual)
}

func TestAuthenticateRequestWrongHeader(t *testing.T) {
	require.NoError(t, setRequestSaltFromBase64([]byte(TEST_SALT_BASE64)))
	body := []byte("my data")
	req, err := http.NewRequest("POST", "http://invalid.", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set(REQUEST_AUTH_HASH_HEADER, INVALID_BASE64)
	actual, err := AuthenticateRequest(req)
	require.Error(t, err)
	expect.Contains(t, err.Error(), "did not match")
	// Still returns body even though there was an authentication error.
	expect.Equal(t, body, actual)
}
