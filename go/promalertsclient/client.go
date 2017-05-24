package promalertsclient

// This package contains a simple client to get alerts from
// a Prometheus alerts manager web endpoint.

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prometheus/alertmanager/dispatch"
	"go.skia.org/infra/go/util"
)

const (
	// We directly access alerts from GCE instance to GCE instance
	// so https is not required.
	API_BASE_PATH_PATTERN = "http://%s/api/v1/alerts"
)

// APIClient is a client to the Prometheus alerts manager web endpoint.
type APIClient interface {
	// GetAlerts fetches all alerts from the server and returns them in a slice.
	// If filter is non-nil, it will be applied to all alerts. If filter
	// returns true, it will be included in the slice.
	GetAlerts(filter func(dispatch.APIAlert) bool) ([]dispatch.APIAlert, error)
}

// apiclient fulfils the APIClient interface
type apiclient struct {
	hc       HTTPClient
	basePath string
}

// HTTPClient represents the http calls needed by the client. This interface
// is a subset of http.Client and makes for easier mocking.
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

// New creates a new APIClient with the given parameters.
func New(hc HTTPClient, server string) APIClient {
	path := fmt.Sprintf(API_BASE_PATH_PATTERN, server)
	return &apiclient{
		hc:       hc,
		basePath: path,
	}
}

// alertResponse represents how Prometheus structures its response to the
// API call.
type alertsResponse struct {
	Status string              `json:"status"`
	Data   []dispatch.APIAlert `json:"data"`
}

// See the APIClient interface for a description of GetAlerts
func (a *apiclient) GetAlerts(filter func(dispatch.APIAlert) bool) ([]dispatch.APIAlert, error) {
	r, err := a.hc.Get(a.basePath)
	if err != nil {
		return nil, err
	}
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error %s", r.Status)
	}
	defer util.Close(r.Body)
	d := json.NewDecoder(r.Body)
	var alerts alertsResponse
	if err := d.Decode(&alerts); err != nil {
		return nil, fmt.Errorf("Could not parse JSON: %s", err)
	}

	retVal := []dispatch.APIAlert{}
	for _, a := range alerts.Data {
		if filter == nil || filter(a) {
			retVal = append(retVal, a)
		}
	}

	return retVal, nil
}
