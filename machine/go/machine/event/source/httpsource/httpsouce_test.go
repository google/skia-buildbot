// Package httpsource implements event.Source by accepting incoming HTTP
// requests that contain a machine.Event serialized as JSON.
package httpsource

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/machine/go/machine"
)

func TestHTTPServer_ValidRequestTriggersEvent(t *testing.T) {
	source, err := New()
	require.NoError(t, err)
	outgoing, err := source.Start(context.Background())
	require.NoError(t, err)

	event := machine.NewEvent()
	event.Host = machine.Host{
		Name: "skia-rpi2-rack4-shelf1-020",
	}

	b, err := json.Marshal(event)
	require.NoError(t, err)
	body := bytes.NewReader(b)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", body)
	source.ServeHTTP(w, r)
	eventFromCh := <-outgoing
	assertdeep.Equal(t, eventFromCh, event)
}
