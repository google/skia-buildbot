package client

import (
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	culprit "go.skia.org/infra/perf/go/culprit/proto/v1"
	pinpoint "go.skia.org/infra/pinpoint/proto/v1"
	grpc "google.golang.org/grpc"
	localCreds "google.golang.org/grpc/credentials/local"
)

// getBackendHostUrl returns the host url for the backend service.
func getBackendHostUrl() string {
	return config.Config.BackendServiceHostUrl
}

// isBackendEnabled returns true if a backend service is enabled for the current instance.
func isBackendEnabled() bool {
	return config.Config.BackendServiceHostUrl != ""
}

// getGrpcConnection returns a ClientConn object that can be used to create individual
// service clients for the BE service.
func getGrpcConnection() (*grpc.ClientConn, error) {
	backendServiceUrl := getBackendHostUrl()
	// TODO(ashwinpv): Explore the use of opentracing with something
	// like https://github.com/grpc-ecosystem/grpc-opentracing/tree/master/go/otgrpc
	conn, err := grpc.Dial(backendServiceUrl, grpc.WithTransportCredentials(localCreds.NewCredentials()))
	if err != nil {
		sklog.Errorf("Error connecting to Backend service at %s: %s", backendServiceUrl, err)
		return nil, err
	}

	return conn, nil
}

// NewPinpointClient returns a new instance of a client for the pinpoint service.
func NewPinpointClient() (pinpoint.PinpointClient, error) {
	if !isBackendEnabled() {
		return nil, skerr.Fmt("Backend service is not enabled for this instance.")
	}

	conn, err := getGrpcConnection()
	if err != nil {
		return nil, err
	}

	return pinpoint.NewPinpointClient(conn), nil
}

// NewCulpritServiceClient returns a new instance of a client for the culprit service.
func NewCulpritServiceClient() (culprit.CulpritServiceClient, error) {
	if !isBackendEnabled() {
		return nil, skerr.Fmt("Backend service is not enabled for this instance.")
	}

	conn, err := getGrpcConnection()
	if err != nil {
		return nil, err
	}

	return culprit.NewCulpritServiceClient(conn), nil
}
