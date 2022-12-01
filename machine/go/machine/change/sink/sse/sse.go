// Package sse implements sink.Sink using Server-Sent Events.
package sse

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sser"
	"go.skia.org/infra/machine/go/machine/change/sink"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SSE implements Sink.
type SSE struct {
	sserServer *sser.ServerImpl
	handler    http.Handler
	sendMetric metrics2.Counter
}

// New returns a new SSE instance.
func New(ctx context.Context, local bool, namespace, labelSelector string, changeEventSSERPeerPort int) (*SSE, error) {
	var peerFinder sser.PeerFinder
	if !local {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("get in-cluster config: %s", err)
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("get in-cluster clientset: %s", err)
		}

		peerFinder, err = sser.NewPeerFinder(clientset, namespace, labelSelector)
		if err != nil {
			return nil, skerr.Wrapf(err, "construct peef finder")
		}
	} else {
		peerFinder = sser.NewPeerFinderLocalhost()
	}

	sserChangeSink, err := sser.New(changeEventSSERPeerPort, peerFinder)
	if err != nil {
		return nil, skerr.Wrapf(err, "create sser Server")
	}
	if err := sserChangeSink.Start(context.Background()); err != nil {
		return nil, skerr.Wrapf(err, "start SSER server")
	}

	return &SSE{
		sserServer: sserChangeSink,
		handler:    sserChangeSink.ClientConnectionHandler(context.Background()),
		sendMetric: metrics2.GetCounter(sink.MetricName, map[string]string{"type": "http"}),
	}, nil
}

// Send implement Sink.
func (s *SSE) Send(ctx context.Context, machineID string) error {
	// Never send an empty string, as the client may mistake that as being no
	// message.
	//
	// In the future we may be able to replace "update" with the JSON serialized
	// machine.Description, which will avoid a second round trip from TMM to
	// retrieve the machine.Description.
	s.sendMetric.Inc(1)
	return s.sserServer.Send(ctx, machineID, "update")
}

// GetHandler returns an http.Handler that should be hooked up to the URL that
// SSE clients will use to receive events.
func (s *SSE) GetHandler(ctx context.Context) http.Handler {
	return s.sserServer.ClientConnectionHandler(ctx)
}
