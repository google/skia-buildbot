package client

import (
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	anomalygroup "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/config"
	culprit "go.skia.org/infra/perf/go/culprit/proto/v1"
	pinpoint "go.skia.org/infra/pinpoint/proto/v1"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// getBackendHostUrl returns the host url for the backend service.
func getBackendHostUrl() string {
	return config.Config.BackendServiceHostUrl
}

// isBackendEnabled returns true if a backend service is enabled for the current instance.
func isBackendEnabled(urlOverride string) bool {
	return urlOverride != "" || config.Config.BackendServiceHostUrl != ""
}

// getGrpcConnection returns a ClientConn object that can be used to create individual
// service clients for the BE service.
func getGrpcConnection(backendServiceUrlOverride string) (*grpc.ClientConn, error) {
	var backendServiceUrl string
	if backendServiceUrlOverride != "" {
		backendServiceUrl = backendServiceUrlOverride
	} else {
		backendServiceUrl = getBackendHostUrl()
	}

	// TODO(ashwinpv): Explore the use of opentracing with something
	// like https://github.com/grpc-ecosystem/grpc-opentracing/tree/master/go/otgrpc

	// TODO(ashwinpv): Once connection is validated, update this to use token based auth.
	conn, err := grpc.Dial(backendServiceUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		sklog.Errorf("Error connecting to Backend service at %s: %s", backendServiceUrl, err)
		return nil, err
	}

	return conn, nil
}

// NewPinpointClient returns a new instance of a client for the pinpoint service.
func NewPinpointClient(backendServiceUrlOverride string) (pinpoint.PinpointClient, error) {
	if !isBackendEnabled(backendServiceUrlOverride) {
		return nil, skerr.Fmt("Backend service is not enabled for this instance.")
	}

	conn, err := getGrpcConnection(backendServiceUrlOverride)
	if err != nil {
		return nil, err
	}

	return pinpoint.NewPinpointClient(conn), nil
}

// NewAnomalyGroupClient returns a new instance of a client for the anomalygroup service.
func NewAnomalyGroupClient(backendServiceUrlOverride string) (anomalygroup.AnomalyGroupServiceClient, error) {
	if !isBackendEnabled(backendServiceUrlOverride) {
		return nil, skerr.Fmt("Backend service is not enabled for this instance.")
	}

	conn, err := getGrpcConnection(backendServiceUrlOverride)
	if err != nil {
		return nil, err
	}

	return anomalygroup.NewAnomalyGroupServiceClient(conn), nil
}

// NewCulpritServiceClient returns a new instance of a client for the culprit service.
func NewCulpritServiceClient(backendServiceUrlOverride string) (culprit.CulpritServiceClient, error) {
	if !isBackendEnabled(backendServiceUrlOverride) {
		return nil, skerr.Fmt("Backend service is not enabled for this instance.")
	}

	conn, err := getGrpcConnection(backendServiceUrlOverride)
	if err != nil {
		return nil, err
	}

	return culprit.NewCulpritServiceClient(conn), nil
}
