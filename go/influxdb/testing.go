package influxdb

import (
	influx_client "github.com/skia-dev/influxdb/client/v2"
)

type testQueryClient struct {
	mockWrite func(influx_client.BatchPoints) error
}

func (tc *testQueryClient) Query(influx_client.Query) (*influx_client.Response, error) {
	return nil, nil
}
func (tc *testQueryClient) Write(bp influx_client.BatchPoints) error {
	if tc.mockWrite != nil {
		return tc.mockWrite(bp)
	}
	return nil
}

func NewTestClient() *Client {
	return &Client{
		Database:     "",
		influxClient: &testQueryClient{},
	}
}

func NewTestClientWithMockWrite(write func(influx_client.BatchPoints) error) *Client {
	return &Client{
		Database:     "",
		influxClient: &testQueryClient{mockWrite: write},
	}
}
