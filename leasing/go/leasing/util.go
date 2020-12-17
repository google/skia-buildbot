package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/rotations"
)

func GetTrooperEmail(httpClient *http.Client) (string, error) {
	resp, err := httpClient.Get(rotations.InfraGardenerURL)
	if err != nil {
		return "", fmt.Errorf("Error when hitting %s: %s", rotations.InfraGardenerURL, err)
	}
	trooper := struct {
		Username string
	}{}
	if err := json.NewDecoder(resp.Body).Decode(&trooper); err != nil {
		return "", fmt.Errorf("Could not get trooper data: %s", err)
	}

	return trooper.Username, nil
}
