package goldclient

import (
	"context"
	"io"
	"os"

	"go.skia.org/infra/gold-client/go/gcsuploader"
	"go.skia.org/infra/gold-client/go/httpclient"
	"go.skia.org/infra/gold-client/go/imagedownloader"
)

const (
	// ErrorWriterKey is the context key used for the error Writer (Typically StdErr).
	ErrorWriterKey = contextKey("errWriter")
	// LogWriterKey is the context key used for the log Writer (Typically StdOut).
	LogWriterKey = contextKey("logWriter")

	gcsUploaderKey     = contextKey("gcsUploader")
	httpClientKey      = contextKey("httpClient")
	imageDownloaderKey = contextKey("imageDownloader")
	nowSourceKey       = contextKey("nowSource")
)

type contextKey string

// WithContext returns a context with the given authenticated network clients loaded.
// By putting these values on the context, it makes it easier to stub out during tests
// and not require several extra arguments on each function call. Failure to have these set
// will result in panics when the function is called. If values have already been set on this
// context, the new value will be ignored.
func WithContext(ctx context.Context, g gcsuploader.GCSUploader, h httpclient.HTTPClient, i imagedownloader.ImageDownloader, n NowSource) context.Context {
	if v := ctx.Value(gcsUploaderKey); v == nil {
		ctx = context.WithValue(ctx, gcsUploaderKey, g)
	}
	if v := ctx.Value(httpClientKey); v == nil {
		ctx = context.WithValue(ctx, httpClientKey, h)
	}
	if v := ctx.Value(imageDownloaderKey); v == nil {
		ctx = context.WithValue(ctx, imageDownloaderKey, i)
	}
	if v := ctx.Value(nowSourceKey); v == nil {
		ctx = context.WithValue(ctx, nowSourceKey, n)
	}
	return ctx
}

func extractGCSUploader(ctx context.Context) gcsuploader.GCSUploader {
	g, ok := ctx.Value(gcsUploaderKey).(gcsuploader.GCSUploader)
	if !ok || g == nil {
		panic("GCSUploader was not set on context. Did you call WithContext?")
	}
	return g
}

func extractHTTPClient(ctx context.Context) httpclient.HTTPClient {
	h, ok := ctx.Value(httpClientKey).(httpclient.HTTPClient)
	if !ok || h == nil {
		panic("HTTPClient was not set on context. Did you call WithContext?")
	}
	return h
}

func extractImageDownloader(ctx context.Context) imagedownloader.ImageDownloader {
	i, ok := ctx.Value(imageDownloaderKey).(imagedownloader.ImageDownloader)
	if !ok || i == nil {
		panic("ImageDownloader was not set on context. Did you call WithContext?")
	}
	return i
}

func extractNowSource(ctx context.Context) NowSource {
	n, ok := ctx.Value(nowSourceKey).(NowSource)
	if !ok || n == nil {
		panic("NowSource was not set on context. Did you call WithContext?")
	}
	return n
}

func extractLogWriter(ctx context.Context) io.Writer {
	w, ok := ctx.Value(LogWriterKey).(io.Writer)
	if !ok || w == nil {
		return os.Stdout
	}
	return w
}

func extractErrorWriter(ctx context.Context) io.Writer {
	w, ok := ctx.Value(ErrorWriterKey).(io.Writer)
	if !ok || w == nil {
		return os.Stderr
	}
	return w
}
