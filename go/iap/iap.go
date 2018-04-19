// iap implements an http.Handler that validates Identity Aware Proxy (IAP)
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

// Allow is used by IAPHandler to enforce additional restrictions on who has
// access to a site. That is, IAP may restrict access to a domain, i.e.
// google.com, but Allow may further restrict it to an even smaller subset,
// such as members of a group.
type Allow interface {
	// Member returns true if the given email address has access.
	Member(email string) bool
}

const (
	// IAP_PUBLIC_KEY_URL is the URL of the public keys that verify a JWT signed by IAP.
	IAP_PUBLIC_KEY_URL = "https://www.gstatic.com/iap/verify/public_key"

	// EMAIL_HEADER is the header added by this module that will contain the
	// user's email address once the IAP signed JWT has been verified.
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
	aud        string
	client     *http.Client
	handler    http.Handler
	mutex      sync.RWMutex
	keys       map[string]string
	jwtToEmail map[string]string
}

// New returns an *IAPHandler, which implements http.Handler, that validates
// x-goog-iap-jwt-asssertion headers and also handles Load Balancer requests.
//
// It also takes an optional 'allow' parameter, which can be nil, that further
// restricts access beyond what the Identity Aware Proxy enforces.
//
// h - The http.Handler to wrap.
// aud - The audience value. To get aud string values from the GCP Console, go
//   to the Identity-Aware Proxy settings for your project, click More next to
//   the Load Balancer resource, and then select Signed Header JWT Audience.
//   The Signed Header JWT dialog that appears displays the aud claim for the
//   selected resource.
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

	// The GCP HTTPS Load Balancer, as configured by GKE, is as dumb as a stump
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
			kid := token.Header["kid"].(string)
			public_pem, err := i.findKey(kid)
			if err != nil {
				return nil, fmt.Errorf("Failed to find key with 'kid' value %q: %s", kid, err)
			}
			public_key, err := jwt.ParseECPublicKeyFromPEM([]byte(public_pem))
			if err != nil {
				return nil, fmt.Errorf("Failed to parse key from PEM: %s", err)
			}
			return public_key, nil
		})
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to validate JWT.")
			return
		}
		email_b, ok := token.Claims.(jwt.MapClaims)["email"]
		if !ok {
			httputils.ReportError(w, r, nil, "Failed to find email in validated JWT.")
			return
		}
		email, ok = email_b.(string)
		if !ok {
			httputils.ReportError(w, r, nil, "Failed to find email string in validated JWT.")
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
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	if email, ok := i.jwtToEmail[jwtAssertion]; ok {
		return email, nil
	} else {
		return "", errNotFound
	}
}

func (i *IAPHandler) readKey(kid string) (string, bool) {
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	key, ok := i.keys[kid]
	return key, ok
}

func (i *IAPHandler) findKey(kid string) (string, error) {
	key, ok := i.readKey(kid)
	if ok {
		return key, nil
	}
	resp, err := i.client.Get(IAP_PUBLIC_KEY_URL)
	if err != nil {
		return "", fmt.Errorf("Could not retrieve iap public_key: %s", err)
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Could not retrieve iap public_key: %s", resp.Status)
	}
	keys := map[string]string{}
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return "", fmt.Errorf("Could not decode keys: %s", err)
	}

	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.keys = keys

	if key, ok = i.keys[kid]; !ok {
		return "", fmt.Errorf("No key with id %q found.", kid)
	} else {
		return key, nil
	}
}
