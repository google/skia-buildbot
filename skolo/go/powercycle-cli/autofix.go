package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/power/go/gatherer"
)

type downBotsResponse struct {
	List []gatherer.DownBot `json:"list"`
}

func getMatchingCandidates(response []byte, hostname string) ([]string, error) {
	r := downBotsResponse{}
	if err := json.Unmarshal(response, &r); err != nil {
		return nil, fmt.Errorf("Badly formatted json: %s", err)
	}
	rv := []string{}
	for _, b := range r.List {
		if b.HostID == hostname && !b.Silenced {
			if b.Status == gatherer.STATUS_DEVICE_MISSING {
				rv = append(rv, b.BotID+"-device")
			} else {
				rv = append(rv, b.BotID)
			}
		}
	}

	return rv, nil
}

func GetAutoFixCandidates(url string) ([]string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("Could not get hostname: %s", err)
	}

	client := httputils.NewTimeoutClient()
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Problem with GET: %s", err)
	}
	defer util.Close(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Could not read http response: %s", err)
	}

	return getMatchingCandidates(body, hostname)
}
