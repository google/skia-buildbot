package influxdb

import (
	influx_client "github.com/influxdata/influxdb/client/v2"
)

type testQueryClient struct{}

func (tc *testQueryClient) Query(influx_client.Query) (*influx_client.Response, error) {
	return nil, nil
}
func (tc *testQueryClient) Write(influx_client.BatchPoints) error {
	return nil
}

func NewTestClient() *Client {
	return &Client{
		Database:     "",
		influxClient: &testQueryClient{},
	}
}
