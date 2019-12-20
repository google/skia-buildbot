// Package adb is a simple wrapper around calling adb.
package adb

import (
	"bytes"
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

func Test_packageVersion(t *testing.T) {
	tests := []struct {
		name       string
		pkg        string
		resp       string
		want       []string
		wantErrout string
	}{
		{
			name:       "empty",
			pkg:        "com.google.android.gms",
			resp:       ``,
			want:       []string{},
			wantErrout: "",
		},
		{
			name: "simple",
			pkg:  "com.google.android.gms",
			resp: `
			versionCode=8186436 targetSdk=23
			versionName=8.1.86 (2287566-436)
					`,
			want:       []string{"8.1.86"},
			wantErrout: "",
		},
		{
			name: "no trailing whitespace",
			pkg:  "com.google.android.gms",
			resp: `
			versionName=8.1.86`,
			want:       []string{"8.1.86"},
			wantErrout: "",
		},
		{
			name:       "error",
			pkg:        "com.google.android.gms",
			resp:       `Failed to talk to device`,
			want:       []string{},
			wantErrout: "Error: Failed to run adb dumpsys package \"Failed to talk to device\": exit code 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errout := &bytes.Buffer{}
			mock := exec.CommandCollector{}
			mock.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
				if tt.wantErrout == "" {
					cmd.Stdout.Write([]byte(tt.resp))
					return nil
				} else {
					cmd.Stderr.Write([]byte(tt.resp))
					return fmt.Errorf("exit code 1")
				}
			})
			ctx := exec.NewContext(context.Background(), mock.Run)

			if got := packageVersion(ctx, errout, tt.pkg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("packageVersion() = %v, want %v", got, tt.want)
			}
			if gotErrout := errout.String(); gotErrout != tt.wantErrout {
				t.Errorf("packageVersion() = %v, want %v", gotErrout, tt.wantErrout)
			}
		})
	}
}

func TestDimensionsFromProperties(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name       string
		dim        map[string][]string
		want       map[string][]string
		responses  []string
		wantErrout string
	}{
		{
			name: "simple",
			dim:  map[string][]string{},
			want: map[string][]string{
				"android_devices":          []string{"1"},
				"device_gms_core_version":  []string{"8.1.86"},
				"device_os":                []string{"L", "LMY47V"},
				"device_os_flavor":         []string{"google"},
				"device_os_type":           []string{"userdebug"},
				"device_playstore_version": []string{"5.2.13"},
				"device_type":              []string{"Nexus 7 [2012] (grouper)"},
				"os":                       []string{"Android"},
			},
			responses: []string{
				`
[ro.product.manufacturer]: [asus]
[ro.product.model]: [Nexus 7]
[ro.product.name]: [razor]
[ro.build.id]: [LMY47V]
[ro.product.brand]: [google]
[ro.build.type]: [userdebug]
[ro.build.product]: [Nexus 7 [2012] (grouper)]
`,
				`
   versionName=8.1.86 (2287566-436)
`,
				`
   versionName=5.2.13 blah blah blah
`,
			},
			wantErrout: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errout := &bytes.Buffer{}

			rIndex := 0
			mock := exec.CommandCollector{}
			mock.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
				cmd.Stdout.Write([]byte(tt.responses[rIndex]))
				rIndex += 1
				return nil
			})
			ctx := exec.NewContext(context.Background(), mock.Run)

			if got := DimensionsFromProperties(ctx, errout, tt.dim); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DimensionsFromProperties() = %#v, want %#v", got, tt.want)
			}
			if gotErrout := errout.String(); gotErrout != tt.wantErrout {
				t.Errorf("DimensionsFromProperties() = %v, want %v", gotErrout, tt.wantErrout)
			}
		})
	}
}
