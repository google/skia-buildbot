// Package adb is a simple wrapper around calling adb.
package adb

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"go.skia.org/infra/go/exec"
)

func TestProperties(t *testing.T) {
	tests := []struct {
		name    string
		want    map[string]string
		resp    string
		wantErr bool
	}{
		{
			name: "simple",
			want: map[string]string{
				"ro.product.manufacturer": "asus",
				"ro.product.model":        "Nexus 7",
				"ro.product.name":         "razor",
			},
			resp: `
[ro.product.manufacturer]: [asus]
[ro.product.model]: [Nexus 7]
[ro.product.name]: [razor]
			`,
			wantErr: false,
		},
		{
			name:    "empty",
			want:    map[string]string{},
			resp:    ``,
			wantErr: false,
		},
		{
			name:    "on error",
			want:    map[string]string{},
			resp:    `error: no devices/emulators found`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mock := exec.CommandCollector{}
			mock.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
				if !tt.wantErr {
					cmd.Stdout.Write([]byte(tt.resp))
					return nil
				} else {
					cmd.Stderr.Write([]byte(tt.resp))
					return fmt.Errorf("exit code 1")
				}
			})
			ctx := exec.NewContext(context.Background(), mock.Run)
			//   assert.Equal(t, "touch /tmp/file"", exec.DebugString(mock.Commands()[0]))

			got, err := Properties(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Properties() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Properties() = %v, want %v", got, tt.want)
			}
		})
	}
}
