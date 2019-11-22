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

func Test_server_transformHandler(t *testing.T) {
	unittest.LargeTest(t)

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body)
	}))
	defer ts.Close()

	srv := &server{
		client: ts.Client(),
		tx:     simpleTx,
	}
	rec := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "https://dot.skia.org/dot?foo", nil)
	req.Header.Set("Referer", ts.URL)
	assert.NoError(t, err)
	srv.transformHandler(rec, req)
	assert.Equal(t, 200, rec.Result().StatusCode)
	assert.Equal(t, "<svg></svg>", rec.Body.String())

	/*
		type fields struct {
			client *http.Client
		}
		type args struct {
			w http.ResponseWriter
			r *http.Request
		}
		tests := []struct {
			name   string
			fields fields
			args   args
		}{
			// TODO: Add test cases.
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				srv := &server{
					client: tt.fields.client,
				}
				srv.transformHandler(tt.args.w, tt.args.r)
			})
		}
	*/
}
