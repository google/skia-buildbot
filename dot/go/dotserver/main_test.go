package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

const body = `
<!DOCTYPE html>
<html>
<body>
    <details>
		<summary>
		<object type="image/svg+xml" data="https://dot.skia.org/dot?foo"></object>
		</summary>
        <pre>
        graph {
            Hello -- World
        }
        </pre>
    </details>
</body>
</html>
`

func simpleTx(ctx context.Context, format, dotCode string) (string, error) {
	return "<svg></svg>", nil
}

func failingTx(ctx context.Context, format, dotCode string) (string, error) {
	return "", fmt.Errorf("Failed to transform.")
}

func Test_server_transformHandler(t *testing.T) {

	goodTS := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, err := fmt.Fprint(w, body)
		assert.NoError(t, err)
	}))
	defer goodTS.Close()

	allowed := regexp.MustCompile(`https://`)
	notAllowed := regexp.MustCompile(`https://notallowed.example.org`)

	goodRequest, err := http.NewRequest("GET", "https://dot.skia.org/dot?foo", nil)
	assert.NoError(t, err)
	goodRequest.Header.Set("Referer", goodTS.URL)

	goodRequestBadTarget, err := http.NewRequest("GET", "https://dot.skia.org/dot?bar", nil)
	assert.NoError(t, err)
	goodRequestBadTarget.Header.Set("Referer", goodTS.URL)

	goodRequestBadFormat, err := http.NewRequest("GET", "https://dot.skia.org/ls", nil)
	assert.NoError(t, err)
	goodRequestBadTarget.Header.Set("Referer", goodTS.URL)

	goodRequestNoReferer, err := http.NewRequest("GET", "https://dot.skia.org/dot?foo", nil)
	assert.NoError(t, err)

	badTS := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer badTS.Close()

	badRequest, err := http.NewRequest("GET", "https://dot.skia.org/dot?foo", nil)
	assert.NoError(t, err)
	badRequest.Header.Set("Referer", badTS.URL)

	type fields struct {
		client  *http.Client
		tx      transformer
		allowed *regexp.Regexp
	}
	type args struct {
		w *httptest.ResponseRecorder
		r *http.Request
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		statusCode int
		body       string
	}{
		{
			name: "Simple success",
			fields: fields{
				client:  goodTS.Client(),
				tx:      simpleTx,
				allowed: allowed,
			},
			args: args{
				w: httptest.NewRecorder(),
				r: goodRequest,
			},
			statusCode: 200,
			body:       "<svg></svg>",
		},
		{
			name: "Not allowed by domain regexp.",
			fields: fields{
				client:  goodTS.Client(),
				tx:      simpleTx,
				allowed: notAllowed,
			},
			args: args{
				w: httptest.NewRecorder(),
				r: goodRequest,
			},
			statusCode: 404,
			body:       "Not an allowed domain.\n",
		},
		{
			name: "The transformation from the input format to SVG fails.",
			fields: fields{
				client:  goodTS.Client(),
				tx:      failingTx,
				allowed: allowed,
			},
			args: args{
				w: httptest.NewRecorder(),
				r: goodRequest,
			},
			statusCode: 404,
			body:       "Failed to transform.\n",
		},
		{
			name: "Good request, but requested URI not found in source document",
			fields: fields{
				client:  goodTS.Client(),
				tx:      simpleTx,
				allowed: allowed,
			},
			args: args{
				w: httptest.NewRecorder(),
				r: goodRequestBadTarget,
			},
			statusCode: 404,
			body:       "Failed to find requester URL in source document.\n",
		},
		{
			name: "Request transform by an unknown format",
			fields: fields{
				client:  goodTS.Client(),
				tx:      simpleTx,
				allowed: allowed,
			},
			args: args{
				w: httptest.NewRecorder(),
				r: goodRequestBadFormat,
			},
			statusCode: 404,
			body:       "Unknown format.\n",
		},
		{
			name: "Request doesn't contain a referer header.",
			fields: fields{
				client:  goodTS.Client(),
				tx:      simpleTx,
				allowed: allowed,
			},
			args: args{
				w: httptest.NewRecorder(),
				r: goodRequestNoReferer,
			},
			statusCode: 404,
			body:       "Missing Referer header.\n",
		},
		{
			name: "Source server returns error.",
			fields: fields{
				client:  badTS.Client(),
				tx:      simpleTx,
				allowed: allowed,
			},
			args: args{
				w: httptest.NewRecorder(),
				r: badRequest,
			},
			statusCode: 404,
			body:       "Failed to get 200 fetching referring page.\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &server{
				client:  tt.fields.client,
				tx:      tt.fields.tx,
				allowed: tt.fields.allowed,
			}
			srv.transformHandler(tt.args.w, tt.args.r)
			assert.Equal(t, tt.statusCode, tt.args.w.Result().StatusCode)
			assert.Equal(t, tt.body, tt.args.w.Body.String())
		})
	}
}
