package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

const body = `
<!DOCTYPE html>
<html>
<body>
    <details>
        <summary><img src="https://dot.skia.org/dot?foo"></src></summary>
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
	unittest.LargeTest(t)

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body)
	}))
	defer ts.Close()

	r, err := http.NewRequest("GET", "https://dot.skia.org/dot?foo", nil)
	assert.NoError(t, err)
	r.Header.Set("Referer", ts.URL)

	type fields struct {
		client *http.Client
		tx     transformer
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
				client: ts.Client(),
				tx:     simpleTx,
			},
			args: args{
				w: httptest.NewRecorder(),
				r: r,
			},
			statusCode: 200,
			body:       "<svg></svg>",
		},
		{
			name: "Transform fail",
			fields: fields{
				client: ts.Client(),
				tx:     failingTx,
			},
			args: args{
				w: httptest.NewRecorder(),
				r: r,
			},
			statusCode: 404,
			body:       "Failed to transform.\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &server{
				client: tt.fields.client,
				tx:     tt.fields.tx,
			}
			srv.transformHandler(tt.args.w, tt.args.r)
			assert.Equal(t, tt.statusCode, tt.args.w.Result().StatusCode)
			assert.Equal(t, tt.body, tt.args.w.Body.String())
		})
	}
}
