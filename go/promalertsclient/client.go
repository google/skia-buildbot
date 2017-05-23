package promalertsclient

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/util"

	"github.com/prometheus/alertmanager/dispatch"
)

const (
	API_BASE_PATH_PATTERN = "http://%s/api/v1/alerts"
)

type ApiClient interface {
	GetAlerts(group string) ([]dispatch.APIAlert, error)
}

type apiclient struct {
	hc       HTTPClient
	basePath string
}

type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

func New(hc HTTPClient, server string) ApiClient {
	path := fmt.Sprintf(API_BASE_PATH_PATTERN, server)
	return &apiclient{
		hc:       hc,
		basePath: path,
	}
}

type alertsResponse struct {
	Status string              `json:"status"`
	Data   []dispatch.APIAlert `json:"data"`
}

func (a *apiclient) GetAlerts(group string) ([]dispatch.APIAlert, error) {
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
		alertName := string(a.Labels["alertname"])
		if group == "" || alertName == group {
			retVal = append(retVal, a)
		}
	}

	return retVal, nil
}
