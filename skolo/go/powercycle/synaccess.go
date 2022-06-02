package powercycle

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	synaccessUser     = "admin"
	synaccessPassword = "admin"

	powerOffDelaySynaccess = 10 * time.Second
)

type SynaccessConfig struct {
	// IP address of the device, i.e. 192.168.1.10
	Address string `json:"address"`

	// Mapping between device id and port on the PDU. These should be the labels on the physical
	// device (i.e. 1-indexed).
	DevPortMap map[DeviceID]int `json:"ports"`
}

type synaccessClient struct {
	name       string
	conf       *SynaccessConfig
	httpClient *http.Client
}

// newSynaccessController creates a new client to talk to the Synaccess PDUs. If connect is true,
// it makes a request to test the IP address.
func newSynaccessController(ctx context.Context, name string, conf *SynaccessConfig, connect bool) (*synaccessClient, error) {
	client := &synaccessClient{
		name:       name,
		conf:       conf,
		httpClient: httputils.DefaultClientConfig().Client(),
	}
	if connect {
		err := client.ping(ctx)
		if err != nil {
			return nil, skerr.Wrapf(err, "Contacting Synaccess device on port %s", conf.Address)
		}
	}
	return client, nil
}

// DeviceIDs implementst the Controller interface.
func (s *synaccessClient) DeviceIDs() []DeviceID {
	var rv []DeviceID
	for d := range s.conf.DevPortMap {
		rv = append(rv, d)
	}
	return rv
}

// PowerCycle sends up to two HTTP requests to the PDU's API to powercycle the given device.
// It returns an error only if the request fails to connect (it does not fail if it gets a non-2XX
// response).
func (s *synaccessClient) PowerCycle(ctx context.Context, id DeviceID, delayOverride time.Duration) error {
	port, ok := s.conf.DevPortMap[id]
	if !ok {
		return skerr.Fmt("No mapping exists for %s", id)
	}

	// The API for the Synaccess series of PDU is a stateless HTTP based system.
	// https://github.com/synaccess-networks/API-Examples/blob/master/examples.sh
	// The $A3 one seems more consistent across models than the other API.s
	turnOffCmd := s.conf.Address + "/cmd.cgi?" +
		"$A3" + // Command for setting an outlet to off or on
		"%20" + // URL encoded space
		strconv.Itoa(port) +
		"%20" + // URL encoded space
		"0" // turn port off

	if err := s.doGetAndIgnoreResponse(ctx, turnOffCmd); err != nil {
		return skerr.Wrapf(err, "turning off port %d", port)
	}

	delay := powerOffDelaySynaccess
	if delayOverride > 0 {
		delay = delayOverride
	}
	sklog.Infof("Switched %s port %d off. Waiting for %s.", s.name, port, delay)

	time.Sleep(delay)

	turnOnCmd := s.conf.Address + "/cmd.cgi?" +
		"$A3" + // Command for setting an outlet to off or on
		"%20" + // URL encoded space
		strconv.Itoa(port) +
		"%20" + // URL encoded space
		"1" // turn port off

	if err := s.doGetAndIgnoreResponse(ctx, turnOnCmd); err != nil {
		return skerr.Wrapf(err, "turning on port %d", port)
	}
	sklog.Infof("Switched %s port %d on.", s.name, port)
	return nil
}

// ping makes a GET request to the IP address, which should return an index.html (some sort of
// dashboard). We ignore the return value and just error if we cannot connect.
func (s *synaccessClient) ping(ctx context.Context) error {
	return skerr.Wrap(s.doGetAndIgnoreResponse(ctx, s.conf.Address))
}

// doGetAndIgnoreResponse makes a GET request to the specified URL and returns an error if the
// connection could not be made.
func (s *synaccessClient) doGetAndIgnoreResponse(ctx context.Context, url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return skerr.Wrap(err)
	}
	req = req.WithContext(ctx)
	req.SetBasicAuth(synaccessUser, synaccessPassword)
	// We only care if we could not connect. The actual response is not useful.
	_, err = s.httpClient.Do(req)
	return skerr.Wrapf(err, "Making request to %s", url)
}
