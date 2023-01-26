// Package skmetadata provides helper functions to implement the meta data server for the Skolo.
package skmetadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	compute "google.golang.org/api/compute/v1"
)

// ProjectMetadata is an interface which supports retrieval of project-level
// metadata values by key.
type ProjectMetadata interface {
	Get(string) (string, error)
}

// InstanceMetadata is an interface which supports retrieval of instance-level
// metadata values by instance name and key.
type InstanceMetadata interface {
	Get(string, string) (string, error)
}

// ValidateToken returns an error if the given token is not valid.
func ValidateToken(tok *oauth2.Token) error {
	if util.TimeIsZero(tok.Expiry) {
		return fmt.Errorf("Token has no expiration!")
	}
	now := time.Now()
	if now.After(tok.Expiry) {
		// This case is covered by tok.Valid(), but we want to provide a
		// better error message.
		return fmt.Errorf("Token is expired! Expiry: %s; time is now %s.", tok.Expiry, now)
	}
	if !tok.Valid() {
		return fmt.Errorf("Token is invalid!")
	}
	return nil
}

// ServiceAccountToken is a struct used for caching an access token for a
// service account.
type ServiceAccountToken struct {
	filename string
	tok      *oauth2.Token
	mtx      sync.RWMutex
	updateFn func() (*oauth2.Token, error)
	tokenSrc oauth2.TokenSource
}

// TODO(stephana): Once this version works in the Skolo, remove the isKeyFile option below and
// get rid of the old implementation of meta_data_server.

// NewServiceAccountToken returns a ServiceAccountToken based on the contents
// of the given file.
// If 'isKeyFile' is true then the given file is assumed to be the keyfile of a service account
// and it is used to to retrieve short-lived tokens continuously.
// If 'isKeyFile' is false the given file is assumed to contain the token
// (updated by another process) and it will be loaded continuously.
func NewServiceAccountToken(fp string, isKeyFile bool) (*ServiceAccountToken, error) {
	rv := &ServiceAccountToken{
		filename: fp,
	}
	// Set the update function whether the provided file contains a cached token
	// or a service account keyfile.
	rv.updateFn = rv.readTokenFromFile
	if isKeyFile {
		var err error
		rv.tokenSrc, err = auth.NewJWTServiceAccountTokenSource(context.TODO(), "#bogus", fp, "", "", compute.CloudPlatformScope, auth.ScopeUserinfoEmail)
		if err != nil {
			return nil, err
		}
		rv.updateFn = rv.tokenSrc.Token
	}
	return rv, rv.Update()
}

// UpdateFromFile updates the ServiceAccountToken from the given file.
func (t *ServiceAccountToken) Update() error {
	tok, err := t.updateFn()
	if err != nil {
		return err
	}

	// Update the stored token.
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.tok = tok
	sklog.Infof("Updated token: %s", tok.AccessToken[len(tok.AccessToken)-8:])
	return nil
}

// readTokenFromFile opens the file provided to the constructor and reads a token from it.
func (t *ServiceAccountToken) readTokenFromFile() (*oauth2.Token, error) {
	// Read the token from the file.
	contents, err := ioutil.ReadFile(t.filename)
	if err != nil {
		return nil, err
	}
	tok := new(oauth2.Token)
	if err := json.NewDecoder(bytes.NewReader(contents)).Decode(tok); err != nil {
		return nil, err
	}

	// Validate the token.
	if err := ValidateToken(tok); err != nil {
		return nil, err
	}
	return tok, nil
}

// Get returns the current value of the access token.
func (t *ServiceAccountToken) Get() (*oauth2.Token, error) {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	return t.tok, nil
}

// UpdateLoop updates the ServiceAccountToken from the given file on a timer.
func (t *ServiceAccountToken) UpdateLoop(ctx context.Context) {
	// get_oauth2_token runs every 45 minutes, and the tokens are valid for
	// 60 minutes. Reloading the token every 10 minutes ensures that our
	// token is always valid.
	util.RepeatCtx(ctx, 10*time.Minute, func(ctx context.Context) {
		if err := t.Update(); err != nil {
			sklog.Errorf("Failed to update ServiceAccountToken from file: %s", err)
		}
	})
}

// makeInstanceMetadataHandler returns an HTTP handler func which serves
// instance-level metadata.
func makeInstanceMetadataHandler(im InstanceMetadata) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		instance := r.RemoteAddr // TODO(borenet): This is not correct.

		key, ok := mux.Vars(r)["key"]
		if !ok {
			httputils.ReportError(w, nil, "Metadata key is required.", http.StatusInternalServerError)
		}

		sklog.Infof("Instance metadata: %s", key)
		val, err := im.Get(instance, key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write([]byte(val)); err != nil {
			httputils.ReportError(w, nil, "Failed to write response.", http.StatusInternalServerError)
			return
		}
	}
}

// makeProjectMetadataHandler returns an HTTP handler func which serves
// project-level metadata.
func makeProjectMetadataHandler(pm ProjectMetadata) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := mux.Vars(r)["key"]
		if !ok {
			httputils.ReportError(w, nil, "Metadata key is required.", http.StatusInternalServerError)
		}
		sklog.Infof("Project metadata: %s", key)
		val, err := pm.Get(key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write([]byte(val)); err != nil {
			httputils.ReportError(w, nil, "Failed to write response.", http.StatusInternalServerError)
			return
		}
	}
}

// mdHandler adds a handler to the given router for the specified metadata endpoint.
func mdHandler(r *mux.Router, level string, handler http.HandlerFunc) {
	path := fmt.Sprintf(metadata.METADATA_SUB_URL_TMPL, level, "{key}")
	r.HandleFunc(path, handler).Headers(metadata.HEADER_MD_FLAVOR_KEY, metadata.HEADER_MD_FLAVOR_VAL)
	sklog.Infof("%s: %s", level, path)
}

// retrieve this server's IP address(es).
func getMyIP() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	rv := []string{}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			rv = append(rv, ip.String())
		}
	}
	return rv, nil
}

// SetupServer adds handlers to the given router which mimic the API of the GCE
// metadata server.
func SetupServer(r *mux.Router, pm ProjectMetadata, im InstanceMetadata, tokenMapping map[string]*ServiceAccountToken) {
	mdHandler(r, metadata.LEVEL_INSTANCE, makeInstanceMetadataHandler(im))
	mdHandler(r, metadata.LEVEL_PROJECT, makeProjectMetadataHandler(pm))

	myIpAddrs, err := getMyIP()
	if err != nil {
		sklog.Fatal(err)
	}

	// The service account token path does not quite follow the pattern of
	// the other two metadata types.
	r.HandleFunc(metadata.TOKEN_PATH, func(w http.ResponseWriter, r *http.Request) {
		// Find the token for this requester.
		ipAddr := strings.Split(r.RemoteAddr, ":")[0]
		var tok *ServiceAccountToken
		if t, ok := tokenMapping[ipAddr]; ok {
			// 1. We have a token specifically for this IP address.
			tok = t
		} else if t, ok := tokenMapping["self"]; ok && util.In(ipAddr, myIpAddrs) {
			// 2. The request is coming from this machine, and we
			// have a token specifically for that case.
			tok = t
		} else if t, ok := tokenMapping["*"]; ok {
			// 3. We don't know about this IP address specifically,
			// but we have a default token.
			tok = t
		} else {
			// 4. None of the above. Return an error.
			httputils.ReportError(w, fmt.Errorf("Unknown IP address %s and no default token provided.", ipAddr), "Failed to retrieve token.", http.StatusInternalServerError)
			return
		}

		t, err := tok.Get()
		if err != nil {
			httputils.ReportError(w, err, "Failed to obtain key.", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Copied from
		// https://github.com/golang/oauth2/blob/f6093e37b6cb4092101a298aba5d794eb570757f/google/google.go#L185
		res := struct {
			AccessToken  string `json:"access_token"`
			ExpiresInSec int    `json:"expires_in"`
			TokenType    string `json:"token_type"`
		}{
			AccessToken:  t.AccessToken,
			ExpiresInSec: int(t.Expiry.Sub(time.Now()).Seconds()),
			TokenType:    t.TokenType,
		}
		sklog.Infof("Token requested by %s, serving %s", r.RemoteAddr, res.AccessToken[len(res.AccessToken)-8:])
		if err := json.NewEncoder(w).Encode(res); err != nil {
			httputils.ReportError(w, err, "Failed to write response.", http.StatusInternalServerError)
			return
		}
	})
}
