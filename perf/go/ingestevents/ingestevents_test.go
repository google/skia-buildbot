package ingestevents

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
)

func TestCreatePubSubBody(t *testing.T) {
	tests := []struct {
		name    string
		args    *IngestEvent
		wantErr bool
	}{
		{
			name: "empty",
			args: &IngestEvent{
				Params:   []paramtools.Params{},
				ParamSet: paramtools.ParamSet{},
			},
			wantErr: false,
		},
		{
			name: "some data",
			args: &IngestEvent{
				Params:   []paramtools.Params{{"foo": "bar", "baz": "quux"}},
				ParamSet: paramtools.ParamSet{"foo": {"bar"}, "baz": {"quux"}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreatePubSubBody(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePubSubBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			want, err := DecodePubSubBody(got)
			assert.NoError(t, err)
			if !reflect.DeepEqual(tt.args, want) {
				t.Errorf("CreatePubSubBody() = %v, want %v", got, want)
			}
		})
	}
}
