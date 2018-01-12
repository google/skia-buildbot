package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
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
	if time.Now().After(tok.Expiry) {
		// This case is covered by tok.Valid(), but we want to provide a
		// better error message.
		return fmt.Errorf("Token is expired!")
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
}

// NewServiceAccountToken returns a ServiceAccountToken based on the contents
// of the given file.
func NewServiceAccountToken(fp string) (*ServiceAccountToken, error) {
	rv := &ServiceAccountToken{
		filename: fp,
	}
	return rv, rv.Update()
}

// UpdateFromFile updates the ServiceAccountToken from the given file.
func (t *ServiceAccountToken) Update() error {
	// Read the token from the file.
	contents, err := ioutil.ReadFile(t.filename)
	if err != nil {
		return err
	}
	tok := new(oauth2.Token)
	if err := json.NewDecoder(bytes.NewReader(contents)).Decode(tok); err != nil {
		return err
	}

	// Validate the token.
	if err := ValidateToken(tok); err != nil {
		return err
	}

	// Update the stored token.
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.tok = tok
	return nil
}

// Get returns the current value of the access token.
func (t *ServiceAccountToken) Get() (string, error) {
	t.mtx.RLock()
	defer t.mtx.RUnlock()

	// Ensure that the token is valid.
	if err := ValidateToken(t.tok); err != nil {
		return "", err
	}

	return t.tok.AccessToken, nil
}

// UpdateLoop updates the ServiceAccountToken from the given file on a timer.
func (t *ServiceAccountToken) UpdateLoop(freq time.Duration, ctx context.Context) {
	util.RepeatCtx(time.Hour, ctx, func() {
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
			httputils.ReportError(w, r, nil, "Metadata key is required.")
		}

		sklog.Infof("Instance metadata: %s", key)
		val, err := im.Get(instance, key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write([]byte(val)); err != nil {
			httputils.ReportError(w, r, nil, "Failed to write response.")
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
			httputils.ReportError(w, r, nil, "Metadata key is required.")
		}
		sklog.Infof("Project metadata: %s", key)
		val, err := pm.Get(key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write([]byte(val)); err != nil {
			httputils.ReportError(w, r, nil, "Failed to write response.")
			return
		}
	}
}

// mdHandler adds a handler to the given router for the specified metadata endpoint.
func mdHandler(r *mux.Router, level string, handler http.HandlerFunc) {
	path := fmt.Sprintf(METADATA_SUB_URL_TMPL, level, "{key}")
	r.HandleFunc(path, handler).Headers(HEADER_MD_FLAVOR_KEY, HEADER_MD_FLAVOR_VAL)
	sklog.Infof("%s: %s", level, path)
}

// SetupServer adds handlers to the given router which mimic the API of the GCE
// metadata server.
func SetupServer(r *mux.Router, pm ProjectMetadata, im InstanceMetadata, tok *ServiceAccountToken) {
	mdHandler(r, LEVEL_INSTANCE, makeInstanceMetadataHandler(im))
	mdHandler(r, LEVEL_PROJECT, makeProjectMetadataHandler(pm))

	// The service account token path does not quite follow the pattern of
	// the other two metadata types.
	path := fmt.Sprintf(METADATA_URL_PREFIX_TMPL, LEVEL_INSTANCE) + "/service-accounts/default/token"
	r.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		t, err := tok.Get()
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to obtain key.")
			return
		}
		if _, err := w.Write([]byte(t)); err != nil {
			httputils.ReportError(w, r, err, "Failed to write response.")
			return
		}
	})
	sklog.Infof("service-account: %s", path)
}
