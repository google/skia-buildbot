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

type IdentityState int

var (
	errInvalid  = errors.New("Invalid User")
	errNotFound = errors.New("Not Found")
)

const (
	INVALID = "~~invalid~~"

	IAP_PUBLIC_KEY_URL = "https://www.gstatic.com/iap/verify/public_key"
)

type iapHandler struct {
	jwtToEmail map[string]string
	aud        string
	keys       map[string]string
	client     *http.Client
	handler    http.Handler
	mutex      sync.Mutex
}

// TODO Implement whitelist checking.
func New(whitelist []string, aud string, h http.Handler) http.Handler {
	// The GCP HTTPS Load Balancer, as configured by GKE is as dump as a stump
	// and requires that the path '/' always return 200 for healthcheck requests,
	// so we have to look for requests coming from the range of IP addresses
	// reserved for the health checker (which is just copied manually from
	// documentation) and respond differently to those requests.
	return &iapHandler{
		jwtToEmail: map[string]string{},
		aud:        aud,
		keys:       map[string]string{},
		client:     httputils.NewTimeoutClient(),
		handler:    h,
	}
}

func (i *iapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only this func is allowed to set x-user-email.
	r.Header.Set("x-user-email", "")
	jwtAssertion := r.Header.Get("x-goog-iap-jwt-assertion")
	if r.URL.Path == "/" && jwtAssertion == "" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if jwtAssertion == "" {
		httputils.ReportError(w, r, fmt.Errorf("Missing jwt assertion for non-'/' path."), "Identity Aware Proxy has been disabled, all access is denied.")
		return
	}
	email, err := i.getEmail(jwtAssertion)
	if err == errInvalid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	} else if err == errNotFound {
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
			if mapClaims["alg"] == nil {
				return nil, fmt.Errorf("Missing 'alg'.")
			}
			if mapClaims["alg"].(string) != "ES256" {
				return nil, fmt.Errorf("Wrong alg: %s", mapClaims["alg"].(string))
			}
			if mapClaims["kid"] == nil {
				return nil, fmt.Errorf("Missing key id 'kid'.")
			}
			public_pem, err := i.findKey(mapClaims["kid"].(string))
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
		i.setEmail(jwtAssertion, email)
	}
	// TODO Maybe set this in the request Context?
	r.Header.Set("x-user-email", email)
	i.handler.ServeHTTP(w, r)
}

func (i *iapHandler) setEmail(jwtAssertion, email string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.jwtToEmail[jwtAssertion] = email
}

func (i *iapHandler) getEmail(jwtAssertion string) (string, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	email, ok := i.jwtToEmail[jwtAssertion]
	if ok && email != INVALID {
		return email, nil
	}
	if !ok {
		return "", errNotFound
	} else {
		return "", errInvalid
	}
}

func (i *iapHandler) findKey(kid string) (string, error) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	key, ok := i.keys[kid]
	if ok {
		return key, nil
	}
	resp, err := i.client.Get(IAP_PUBLIC_KEY_URL)
	if err != nil || resp.StatusCode != 200 {
		return "", fmt.Errorf("Could not retrieve iap public_key")
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
