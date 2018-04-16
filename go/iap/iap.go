package iap

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
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
)

type iapHandler struct {
	jwtToEmail          map[string]string
	healthCheckIPRanges []*net.IPNet
	aud                 string
	keys                map[string]string
	client              *http.Client
	handler             http.Handler
	mutex               sync.Mutex
}

// TODO All this code and we don't even support setting a list of users or domains to restrict the access to.

func New(allowed []string, projectNumber, backendServiceId string, h http.Handler) (http.Handler, error) {
	// The GCP HTTPS Load Balancer, as configured by GKE is as dump as a stump
	// and requires that the path '/' always return 200 for healthcheck requests,
	// so we have to look for requests coming from the range of IP addresses
	// reserved for the health checker (which is just copied manually from
	// documentation) and respond differently to those requests.
	_, healthCheck1, err := net.ParseCIDR("130.211.0.0/22")
	if err != nil {
		return nil, err
	}
	_, healthCheck2, err := net.ParseCIDR("35.191.0.0/16")
	if err != nil {
		return nil, err
	}
	return &iapHandler{
		jwtToEmail:          map[string]string{},
		healthCheckIPRanges: []*net.IPNet{healthCheck1, healthCheck2},
		aud:                 fmt.Sprintf("/projects/%d/global/backendServices/%s", projectNumber, backendServiceId),
		keys:                map[string]string{},
		client:              httputils.NewTimeoutClient(),
		handler:             h,
	}, nil
}

func (i *iapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only this func is allowed to set x-user-email.
	r.Header.Set("x-user-email", "")
	jwtAssertion := r.Header.Get("x-goog-iap-jwt-assertion")
	if r.URL.Path == "/" && jwtAssertion == "" {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			httputils.ReportError(w, r, fmt.Errorf("Invalid IP:port format for %q: %s", r.RemoteAddr, err), "Failed health check.")
			return
		}

		userIP := net.ParseIP(ip)
		if userIP == nil {
			httputils.ReportError(w, r, fmt.Errorf("Invalid IP format for %q: %s", r.RemoteAddr, err), "Failed health check.")
			return
		}
		for _, ipRange := range i.healthCheckIPRanges {
			if ipRange.Contains(userIP) {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		httputils.ReportError(w, r, fmt.Errorf("Healthcheck request came from invalid IP range %q: %s", r.RemoteAddr, err), "Failed health check.")
		return
	}
	if jwtAssertion == "" {
		httputils.ReportError(w, r, fmt.Errorf("Missing jwt assertion for non-'/' path."), "Identity Aware Proxy has been disabled, all access is denied.")
		return
	}
	email, err := i.getEmail(jwtAssertion)
	if err == errInvalid {
		httputils.ReportError(w, r, fmt.Errorf("Invalid JWT"), "Invalid User")
		return
	} else if err != errNotFound {
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
			if mapClaims["exp"].(string) == "" || mapClaims["iat"].(string) == "" {
				return nil, fmt.Errorf("Missing exp or iat claim.")
			}
			if !mapClaims.VerifyAudience(i.aud, true) {
				return nil, fmt.Errorf("Wrong audience.")
			}
			if !mapClaims.VerifyIssuer("https://cloud.google.com/iap", true) {
				return nil, fmt.Errorf("Wrong issuer.")
			}
			if mapClaims["alg"].(string) != "ES256" {
				return nil, fmt.Errorf("Wrong alg.")
			}
			return i.findKey(mapClaims["kid"].(string))
		})
		if err != nil {
			httputils.ReportError(w, r, fmt.Errorf("Failed to validate JWT: %s", err), "Failed to validate JWT.")
			return
		}
		email = token.Claims.(jwt.MapClaims)["email"].(string)
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
	resp, err := i.client.Get("https://www.gstatic.com/iap/verify/public_key")
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
