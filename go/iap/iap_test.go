package iap

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/mockhttpclient"
)

func TestBasics(t *testing.T) {
	h, err := New([]string{}, "145247227042", "k8s-be-32071--e03569f20c11b77d", nil)
	assert.NoError(t, err)
	ih := h.(*iapHandler)
	assert.Equal(t, "/projects/145247227042/global/backendServices/k8s-be-32071--e03569f20c11b77d", ih.aud)
}

func TestIPinRange(t *testing.T) {
	h, err := New([]string{}, "145247227042", "k8s-be-32071--e03569f20c11b77d", nil)
	assert.NoError(t, err)
	ih := h.(*iapHandler)
	assert.NoError(t, ih.IPinRange("130.211.1.1:80"))
	assert.NoError(t, ih.IPinRange("35.191.1.1:8080"))
	assert.Error(t, ih.IPinRange("130.211.1.1"))
	assert.Error(t, ih.IPinRange("10.1.1.1:80"))
}

func TestFindKey(t *testing.T) {
	h, err := New([]string{}, "145247227042", "k8s-be-32071--e03569f20c11b77d", nil)
	assert.NoError(t, err)
	ih := h.(*iapHandler)

	m := mockhttpclient.NewURLMock()
	m.MockOnce(IAP_PUBLIC_KEY_URL, mockhttpclient.MockGetDialogue([]byte(`{"foo": "bar"}`)))
	m.MockOnce(IAP_PUBLIC_KEY_URL, mockhttpclient.MockGetDialogue([]byte(`{"foo": "bar"}`)))
	m.MockOnce(IAP_PUBLIC_KEY_URL, mockhttpclient.MockGetDialogue([]byte(`{"foo": "bar", "baz": "quxx"}`)))
	ih.client = m.Client()

	// Make request on empty keys and force an http request.
	key, err := ih.findKey("foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", key)
	assert.Len(t, ih.keys, 1)

	// Make request on key that is in cache, so http request.
	key, err = ih.findKey("foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", key)
	assert.Len(t, ih.keys, 1)

	// Make request for missing key and force an http request, and the key isn't there.
	_, err = ih.findKey("baz")
	assert.Error(t, err)
	assert.Len(t, ih.keys, 1)

	// Make request for missing key and force an http request, and the key is now present.
	key, err = ih.findKey("baz")
	assert.NoError(t, err)
	assert.Equal(t, "quxx", key)
	assert.Len(t, ih.keys, 2)
}

func TestEmail(t *testing.T) {
	h, err := New([]string{}, "145247227042", "k8s-be-32071--e03569f20c11b77d", nil)
	assert.NoError(t, err)
	ih := h.(*iapHandler)

	email, err := ih.getEmail("a.b.c")
	assert.Equal(t, errNotFound, err)

	ih.setEmail("a.b.c", "fred@example.org")

	email, err = ih.getEmail("a.b.c")
	assert.NoError(t, err)
	assert.Equal(t, "fred@example.org", email)

	ih.setEmail("x.y.z", INVALID)
	email, err = ih.getEmail("x.y.z")
	assert.Equal(t, errInvalid, err)
}

func TestServe(t *testing.T) {
	h, err := New([]string{}, "145247227042", "k8s-be-32071--e03569f20c11b77d", nil)
	assert.NoError(t, err)
	ih := h.(*iapHandler)

	ih.setEmail("a.b.c", "fred@example.org")

	// Healthcheck from wrong address.
	r := httptest.NewRequest("GET", "http://example.com/", nil)
	r.RemoteAddr = "10.0.0.1:8080"
	w := httptest.NewRecorder()
	ih.ServeHTTP(w, r)
	resp := w.Result()
	assert.Equal(t, 500, resp.StatusCode)

	// Good healtcheck.
	r.RemoteAddr = "130.211.1.1:8080"
	w = httptest.NewRecorder()
	ih.ServeHTTP(w, r)
	resp = w.Result()
	assert.Equal(t, 200, resp.StatusCode)

	// Add a user that has already been validated.
	ih.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "<html><body>Hello World!</body></html>")
	})
	ih.setEmail("a.b.c", "fred@example.com")
	r = httptest.NewRequest("GET", "http://example.com/foo", nil)
	r.Header.Set("x-goog-iap-jwt-assertion", "a.b.c")
	w = httptest.NewRecorder()
	ih.ServeHTTP(w, r)
	resp = w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "<html><body>Hello World!</body></html>", string(body))
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "fred@example.com", r.Header.Get("x-user-email"))

	// Add a user that has already been found to be invalid.
	ih.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "<html><body>Hello World!</body></html>")
	})
	ih.setEmail("x.y.z", INVALID)
	r = httptest.NewRequest("GET", "http://example.com/foo", nil)
	r.Header.Set("x-goog-iap-jwt-assertion", "x.y.z")
	w = httptest.NewRecorder()
	ih.ServeHTTP(w, r)
	resp = w.Result()
	assert.Equal(t, 401, resp.StatusCode)
	assert.Equal(t, "", r.Header.Get("x-user-email"))
}

// Test the full happy path by signing our own token and verifying it.
func TestSigning(t *testing.T) {
	h, err := New([]string{}, "145247227042", "k8s-be-32071--e03569f20c11b77d", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "<html><body>Hello World!</body></html>")
	}))
	assert.NoError(t, err)
	ih := h.(*iapHandler)

	// Create a new token object, specifying signing method and the claims
	// you would like it to contain.
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"kid":   "foo",
		"alg":   "ES256",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
		"aud":   "/projects/145247227042/global/backendServices/k8s-be-32071--e03569f20c11b77d",
		"iss":   "https://cloud.google.com/iap",
		"nbf":   time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
		"email": "test@example.org",
	})

	// The test public and private keys were generated by running:
	// $ openssl ecparam -genkey -name prime256v1 -noout -out ec_private.pem
	// $ openssl ec -in ec_private.pem -pubout -out ec_public.pem

	// We obfuscate the private key to avoid getting flagged by scanners looking
	// for actual private keys that need to be kept private.
	private := fmt.Sprintf(`-----BEGIN EC %s KEY-----
MHcCAQEEIF/CmKlaP9rhGZi4xbhun+xpLjYHrux57KoLilrYwYqzoAoGCCqGSM49
AwEHoUQDQgAE8JvCczoQZVRKtlbQvaaxcT7OJX7QlMgnmZhQXYTaxTcUmaV2zxD/
U3fSoPyzliWdVHK6Zc5wW6kYVYuT5e9/Kg==
-----END EC %s KEY-----
`, "PRIVATE", "PRIVATE")
	public := `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8JvCczoQZVRKtlbQvaaxcT7OJX7Q
lMgnmZhQXYTaxTcUmaV2zxD/U3fSoPyzliWdVHK6Zc5wW6kYVYuT5e9/Kg==
-----END PUBLIC KEY-----
`
	// Parse the private key.
	key, err := jwt.ParseECPrivateKeyFromPEM([]byte(private))
	assert.NoError(t, err)

	// Sign and get the complete encoded token as a string using the private key.
	tokenString, err := token.SignedString(key)
	assert.NoError(t, err)

	// Mock out the request for the associated public key.
	m := mockhttpclient.NewURLMock()
	public_keys := map[string]string{"foo": string(public)}
	public_keys_json, err := json.Marshal(public_keys)
	assert.NoError(t, err)
	m.Mock(IAP_PUBLIC_KEY_URL, mockhttpclient.MockGetDialogue([]byte(public_keys_json)))
	ih.client = m.Client()

	r := httptest.NewRequest("GET", "http://example.com/foo", nil)
	r.Header.Set("x-goog-iap-jwt-assertion", tokenString)
	w := httptest.NewRecorder()
	ih.ServeHTTP(w, r)
	resp := w.Result()
	body, _ := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "<html><body>Hello World!</body></html>", string(body))
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "test@example.org", r.Header.Get("x-user-email"))

	// Now remove claims one at a time and confirm we get a 500 each time.
	for _, remove := range []string{"kid", "alg", "iat", "exp", "aud", "iss", "email"} {
		claims := jwt.MapClaims{
			"kid":   "foo",
			"alg":   "ES256",
			"iat":   now.Unix(),
			"exp":   now.Add(time.Hour).Unix(),
			"aud":   "/projects/145247227042/global/backendServices/k8s-be-32071--e03569f20c11b77d",
			"iss":   "https://cloud.google.com/iap",
			"nbf":   time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
			"email": "test@example.org",
		}
		delete(claims, remove)

		token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
		tokenString, err := token.SignedString(key)
		assert.NoError(t, err)
		r := httptest.NewRequest("GET", "http://example.com/foo", nil)
		r.Header.Set("x-goog-iap-jwt-assertion", tokenString)
		w := httptest.NewRecorder()
		ih.ServeHTTP(w, r)
		resp := w.Result()
		assert.Equal(t, 500, resp.StatusCode)
	}

}
