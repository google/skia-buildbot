package swarming

import (
	"fmt"
	"net/http"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
)

const (
	AUTH_SCOPE    = "https://www.googleapis.com/auth/userinfo.email"
	API_BASE_PATH = "https://chromium-swarm.appspot.com/_ah/api/swarming/v1/"

	DIMENSION_POOL_KEY   = "pool"
	DIMENSION_POOL_VALUE = "Skia"
)

// ApiClient is a Skia-specific wrapper around the Swarming API.
type ApiClient struct {
	s *swarming.Service
}

// NewApiClient returns an ApiClient instance which uses the given authenticated
// http.Client.
func NewApiClient(c *http.Client) (*ApiClient, error) {
	s, err := swarming.New(c)
	if err != nil {
		return nil, err
	}
	s.BasePath = API_BASE_PATH
	return &ApiClient{s}, nil
}

// ListSkiaBots returns a slice of swarming.SwarmingRpcsBotInfo instances
// corresponding to the Skia Swarming bots.
func (c *ApiClient) ListSkiaBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	bots := []*swarming.SwarmingRpcsBotInfo{}
	cursor := ""
	for {
		call := c.s.Bots.List()
		call.Dimensions(fmt.Sprintf("%s:%s", DIMENSION_POOL_KEY, DIMENSION_POOL_VALUE))
		call.Limit(100)
		if cursor != "" {
			call.Cursor(cursor)
		}
		res, err := call.Do()
		if err != nil {
			return nil, err
		}
		bots = append(bots, res.Items...)
		if len(res.Items) == 0 || res.Cursor == "" {
			break
		}
		cursor = res.Cursor
	}

	return bots, nil
}
