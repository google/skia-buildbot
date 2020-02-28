package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	SKIA_TROOPER_URL = "http://tree-status.skia.org/current-trooper"
)

func GetTrooperEmail(httpClient *http.Client) (string, error) {
	resp, err := httpClient.Get(SKIA_TROOPER_URL)
	if err != nil {
		return "", fmt.Errorf("Error when hitting %s: %s", SKIA_TROOPER_URL, err)
	}
	trooper := struct {
		Username string
	}{}
	if err := json.NewDecoder(resp.Body).Decode(&trooper); err != nil {
		return "", fmt.Errorf("Could not get trooper data: %s", err)
	}

	return trooper.Username, nil
}
