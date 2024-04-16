package client

import (
	"context"
	"crypto/tls"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	anomalygroup "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/config"
	culprit "go.skia.org/infra/perf/go/culprit/proto/v1"
	pinpoint "go.skia.org/infra/pinpoint/proto/v1"
	"golang.org/x/oauth2/google"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
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
func getGrpcConnection(backendServiceUrlOverride string, insecure_conn bool) (*grpc.ClientConn, error) {
	var backendServiceUrl string
	if backendServiceUrlOverride != "" {
		backendServiceUrl = backendServiceUrlOverride
	} else {
		backendServiceUrl = getBackendHostUrl()
	}

	// TODO(ashwinpv): Explore the use of opentracing with something
	// like https://github.com/grpc-ecosystem/grpc-opentracing/tree/master/go/otgrpc

	opts := []grpc.DialOption{}
	if insecure_conn {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsCreds := credentials.NewTLS(&tls.Config{
			// Since the communication is internal within the same GKE cluster,
			// we do not need to verify the server certificate.
			InsecureSkipVerify: true,
		})
		tokenSource, err := getOauthCredentials(context.Background())
		if err != nil {
			return nil, skerr.Wrapf(err, "Error getting token source")
		}
		opts = append(opts, grpc.WithTransportCredentials(tlsCreds), grpc.WithPerRPCCredentials(tokenSource))
	}

	conn, err := grpc.Dial(backendServiceUrl, opts...)
	if err != nil {
		sklog.Errorf("Error connecting to Backend service at %s: %s", backendServiceUrl, err)
		return nil, err
	}

	return conn, nil
}

// getOauthCredentials returns a token source that will provide oauth tokens
// for the service account running the client process.
func getOauthCredentials(ctx context.Context) (oauth.TokenSource, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)

	if err != nil {
		return oauth.TokenSource{}, skerr.Wrapf(err, "Failed to create oauth token source.")
	}
	return oauth.TokenSource{TokenSource: tokenSource}, nil
}

// NewPinpointClient returns a new instance of a client for the pinpoint service.
func NewPinpointClient(backendServiceUrlOverride string) (pinpoint.PinpointClient, error) {
	if !isBackendEnabled(backendServiceUrlOverride) {
		return nil, skerr.Fmt("Backend service is not enabled for this instance.")
	}

	conn, err := getGrpcConnection(backendServiceUrlOverride, false)
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

	conn, err := getGrpcConnection(backendServiceUrlOverride, false)
	if err != nil {
		return nil, err
	}

	return anomalygroup.NewAnomalyGroupServiceClient(conn), nil
}

// NewCulpritServiceClient returns a new instance of a client for the culprit service.
func NewCulpritServiceClient(backendServiceUrlOverride string, insecure_conn bool) (culprit.CulpritServiceClient, error) {
	if !isBackendEnabled(backendServiceUrlOverride) {
		return nil, skerr.Fmt("Backend service is not enabled for this instance.")
	}

	conn, err := getGrpcConnection(backendServiceUrlOverride, insecure_conn)
	if err != nil {
		return nil, err
	}

	return culprit.NewCulpritServiceClient(conn), nil
}
