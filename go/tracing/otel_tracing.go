package tracing

import (
	gcp "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/bridge/opencensus"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.skia.org/infra/go/skerr"
)

func InitializeOtel() error {
	// No ProjectID options means that it finds default credentials
	// and sets project based on those creds.
	// https://github.com/GoogleCloudPlatform/opentelemetry-operations-go/blob/main/exporter/trace/cloudtrace.go#L173
	exporter, err := gcp.New()
	if err != nil {
		return skerr.Wrap(err)
	}

	provider := trace.NewTracerProvider(trace.WithBatcher(exporter))
	otel.SetTracerProvider(provider)

	opencensus.InstallTraceBridge(opencensus.WithTracerProvider(provider))
	return nil
}
