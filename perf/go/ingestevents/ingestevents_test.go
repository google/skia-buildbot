package ingestevents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
)

func TestCreatePubSubBody(t *testing.T) {
	tests := []struct {
		name string
		args *IngestEvent
	}{
		{
			name: "nils",
			args: &IngestEvent{
				TraceIDs: nil,
				ParamSet: nil,
			},
		},
		{
			name: "empty",
			args: &IngestEvent{
				TraceIDs: []string{},
				ParamSet: paramtools.NewReadOnlyParamSet(),
			},
		},
		{
			name: "some data",
			args: &IngestEvent{
				TraceIDs: []string{",foo=bar,baz=quux,"},
				ParamSet: paramtools.NewReadOnlyParamSet(paramtools.Params{"foo": "bar", "baz": "quux"}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the IngestEvents round trip correctly.
			encoded, err := CreatePubSubBody(tt.args)
			assert.NoError(t, err)
			want, err := DecodePubSubBody(encoded)
			assert.NoError(t, err)
			assert.Equal(t, want, tt.args)
		})
	}
}
