package common

import (
	"go.skia.org/infra/go/skerr"
	"go.temporal.io/sdk/client"
)

type TemporalProvider interface {
	// NewClient returns a Temporal Client and a clean up function
	NewClient(string, string) (client.Client, func(), error)
}

type DefaultTemporalProvider struct {
}

// NewClient implements TemporalProvider.NewClient
func (DefaultTemporalProvider) NewClient(hostPort string, namespace string) (client.Client, func(), error) {
	c, err := client.NewLazyClient(client.Options{
		HostPort:  hostPort,
		Namespace: namespace,
	})

	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	return c, func() {
		c.Close()
	}, nil
}
