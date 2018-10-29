package alertclient

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.skia.org/infra/am/go/incident"
	"go.skia.org/infra/am/go/silence"
	"go.skia.org/infra/go/util"
)

const (
	// We directly access alerts from Kubernetes pod to pod
	// so https is not required.
	API_INCIDENTS_PATTERN = "http://%s/_/incidents"
	API_SILENCES_PATTERN  = "http://%s/_/silences"
)

type APIClient interface {
	// GetAlerts fetches all alerts from the server and returns them in a slice.
	GetAlerts() ([]incident.Incident, error)
	// GetSilences fetches all silences from the server and returns them in a slice.
	GetSilences() ([]silence.Silence, error)
}

// apiclient fulfills the APIClient interface
type apiclient struct {
	hc     HTTPClient
	server string
}

// HTTPClient represents the http calls needed by the client. This interface
// is a subset of http.Client and makes for easier mocking.
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

// New creates a new APIClient with the given parameters.
func New(hc HTTPClient, server string) *apiclient {
	return &apiclient{
		hc:     hc,
		server: server,
	}
}

// See the APIClient interface for a description of GetAlerts
func (a *apiclient) GetAlerts() ([]incident.Incident, error) {
	r, err := a.hc.Get(fmt.Sprintf(API_INCIDENTS_PATTERN, a.server))
	if err != nil {
		return nil, err
	}
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error %s", r.Status)
	}
	defer util.Close(r.Body)
	var alerts []incident.Incident
	if err := json.NewDecoder(r.Body).Decode(&alerts); err != nil {
		return nil, fmt.Errorf("Could not parse JSON: %s", err)
	}
	return alerts, nil
}

// See the APIClient interface for a description of GetSilences
func (a *apiclient) GetSilences() ([]silence.Silence, error) {
	r, err := a.hc.Get(fmt.Sprintf(API_SILENCES_PATTERN, a.server))
	if err != nil {
		return nil, err
	}
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP error %s", r.Status)
	}
	defer util.Close(r.Body)
	var silences []silence.Silence
	if err := json.NewDecoder(r.Body).Decode(&silences); err != nil {
		return nil, fmt.Errorf("Could not parse JSON: %s", err)
	}
	return silences, nil
}
