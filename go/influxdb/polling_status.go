package influxdb

import (
	"time"

	"go.skia.org/infra/go/util"
)

func makePollFn(query string, dbClient *Client) func(interface{}) error {
	return func(value interface{}) error {
		if err := dbClient.Query(&value, query); err != nil {
			return err
		}
		return nil
	}
}

// PollingStatus is a convenience struct used for periodically querying
// InfluxDB.
func NewPollingStatus(value interface{}, query string, dbClient *Client) (*util.PollingStatus, error) {
	return util.NewPollingStatus(value, makePollFn(query, dbClient), time.Minute)
}

// IntPollingStatus is a wrapper around PollingStatus which expects an
// integer value from InfluxDB.
func NewIntPollingStatus(query string, dbClient *Client) (*util.IntPollingStatus, error) {
	return util.NewIntPollingStatus(makePollFn(query, dbClient), time.Minute)
}
