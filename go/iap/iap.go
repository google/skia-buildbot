// iap implements an http.Handler that validates Identity Aware Proxy
// requests: https://cloud.google.com/iap/docs/signed-headers-howto
package iap

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	jwt "github.com/dgrijalva/jwt-go"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
)

var (
	errNotFound = errors.New("Not Found")
)

// Allow is used by IAPHandler to enforce additional restrictions
// on who has access to a site.
type Allow interface {
	// Member returns true if the given email address has access.
	Member(email string) bool
}

const (
	// IAP_PUBLIC_KEY_URL is the URL of the public keys that iap uses to sign JWTs.
	IAP_PUBLIC_KEY_URL = "https://www.gstatic.com/iap/verify/public_key"

	// EMAIL_HEADER is the header that will contain the user's email address.
	EMAIL_HEADER = "x-user-email"
)

// IAPHandler implements an http.Handler that validates x-goog-iap-jwt-asssertion
// headers and also handles Load Balancer requests.
//
// It also can optionally further restrict access beyond what the Identity
// Aware Proxy enforces.
//
// A successfully validated request will contain an EMAIL_HEADER with the user's
// email address.
type IAPHandler struct {
	allow      Allow
	jwtToEmail map[string]string
	aud        string
	keys       map[string]string
	client     *http.Client
	handler    http.Handler
	mutex      sync.Mutex
}

// New returns an *IAPHandler, which implements http.Handler, that validates
// x-goog-iap-jwt-asssertion headers and also handles Load Balancer requests.
//
// It also takes an optional 'allowed' parameter, which can be nil, that
// further restricts access beyond what the Identity Aware Proxy enforces.
func New(h http.Handler, aud string, allow Allow) *IAPHandler {
	return &IAPHandler{
		allow:      allow,
		jwtToEmail: map[string]string{},
		aud:        aud,
		keys:       map[string]string{},
		client:     httputils.NewTimeoutClient(),
		handler:    h,
	}
}

func (i *IAPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only this func is allowed to set x-user-email.
	r.Header.Set(EMAIL_HEADER, "")
	jwtAssertion := r.Header.Get("x-goog-iap-jwt-assertion")

	// The GCP HTTPS Load Balancer, as configured by GKE, is as dump as a stump
	// and requires that the path '/' always return 200 for healthcheck requests,
	// so we have to look for requests for '/' that have no x-goog-iap-assertion
	// header and respond with a bodyless 200 OK.
	if r.URL.Path == "/" && jwtAssertion == "" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if jwtAssertion == "" {
		httputils.ReportError(w, r, fmt.Errorf("Missing jwt assertion for non-'/' path."), "Identity Aware Proxy has been disabled, all access is denied.")
		return
	}
	email, err := i.getEmail(jwtAssertion)
	// A user may have been previously allowed, but has since been removed from
	// the list, so check here.
	if err == nil && i.allow != nil && !i.allow.Member(email) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err != nil {
		// Validate jwtAsssertion
		parser := &jwt.Parser{
			ValidMethods: []string{"ES256"},
		}
		token, err := parser.Parse(jwtAssertion, func(token *jwt.Token) (interface{}, error) {
			mapClaims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return nil, fmt.Errorf("Doesn't contain MapClaims.")
			}
			if mapClaims.Valid() != nil {
				return nil, fmt.Errorf("Invalid claims.")
			}
			if mapClaims["exp"] == nil || mapClaims["iat"] == nil {
				return nil, fmt.Errorf("Missing exp or iat claim.")
			}
			if !mapClaims.VerifyAudience(i.aud, true) {
				return nil, fmt.Errorf("Wrong audience.")
			}
			if !mapClaims.VerifyIssuer("https://cloud.google.com/iap", true) {
				return nil, fmt.Errorf("Wrong issuer.")
			}
			if mapClaims["hd"] == nil || mapClaims["hd"].(string) != "google.com" {
				return nil, fmt.Errorf("Missing or incorrect hd claims: %v", mapClaims["hd"])
			}
			if token.Header["alg"] == nil {
				return nil, fmt.Errorf("Missing 'alg'.")
			}
			if token.Header["alg"].(string) != "ES256" {
				return nil, fmt.Errorf("Wrong alg: %s", token.Header["alg"].(string))
			}
			if token.Header["kid"] == nil {
				return nil, fmt.Errorf("Missing key id 'kid'.")
			}
			public_pem, err := i.findKey(token.Header["kid"].(string))
			if err != nil {
				return nil, fmt.Errorf("Failed to find key with 'kid': %s", err)
			}
			public_key, err := jwt.ParseECPublicKeyFromPEM([]byte(public_pem))
			if err != nil {
				return nil, fmt.Errorf("Failed to parse key from PEM: %s", err)
			}
			return public_key, nil
		})
		if err != nil {
			httputils.ReportError(w, r, fmt.Errorf("Failed to validate JWT: %s", err), "Failed to validate JWT.")
			return
		}
		email_b, ok := token.Claims.(jwt.MapClaims)["email"]
		if !ok {
			httputils.ReportError(w, r, fmt.Errorf("Failed to find email in validated JWT: %s", err), "Failed to find email in validated JWT.")
			return
		}
		email, ok = email_b.(string)
		if !ok {
			httputils.ReportError(w, r, fmt.Errorf("Failed to find email as string in validated JWT: %s", err), "Failed to find email string in validated JWT.")
			return
		}
		if i.allow != nil && !i.allow.Member(email) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		i.setEmail(jwtAssertion, email)
	}
	r.Header.Set(EMAIL_HEADER, email)
	i.handler.ServeHTTP(w, r)
}

func (i *IAPHandler) setEmail(jwtAssertion, email string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.jwtToEmail[jwtAssertion] = email
}

func (i *IAPHandler) getEmail(jwtAssertion string) (string, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	if email, ok := i.jwtToEmail[jwtAssertion]; ok {
		return email, nil
	} else {
		return "", errNotFound
	}
}

func (i *IAPHandler) findKey(kid string) (string, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	key, ok := i.keys[kid]
	if ok {
		return key, nil
	}
	resp, err := i.client.Get(IAP_PUBLIC_KEY_URL)
	if err != nil {
		return "", fmt.Errorf("Could not retrieve iap public_key: %s", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Could not retrieve iap public_key: %s", resp.Status)
	}
	defer util.Close(resp.Body)
	keys := map[string]string{}
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return "", fmt.Errorf("Could not decode keys: %s", err)
	}
	i.keys = keys
	if key, ok = i.keys[kid]; ok {
		return key, nil
	}
	return "", fmt.Errorf("No key with id %q found.", kid)
}
