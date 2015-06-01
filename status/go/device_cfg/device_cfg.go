package device_cfg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"reflect"
	"time"

	"go.skia.org/infra/go/gitiles"
	"go.skia.org/infra/go/util"
)

// downloadAndExecPython downloads a Python script from the Repo, executes it, and
// parses its output in JSON format into the given destination.
func downloadAndExecPython(r *gitiles.Repo, dst interface{}, srcPath string, workdir string) error {
	destDir, err := ioutil.TempDir(workdir, "gitiles")
	if err != nil {
		return err
	}
	defer util.RemoveAll(destDir)
	destPath := path.Join(destDir, path.Base(srcPath))
	if err := r.DownloadFile(srcPath, destPath); err != nil {
		return err
	}
	output, err := exec.Command("python", destPath).Output()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(output, dst); err != nil {
		return err
	}
	return nil
}

// AndroidDeviceCfg represents configuration information for an Android device.
type AndroidDeviceCfg struct {
	Serial         string `json:"serial"`
	AndroidSDKRoot string `json:"androidSDKRoot"`
	HasRoot        bool   `json:"hasRoot"`
}

func (c *AndroidDeviceCfg) UnmarshalJSON(data []byte) error {
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	if len(arr) != 3 {
		return fmt.Errorf("Expected: [<string>, <string>, <bool>]")
	}
	if reflect.TypeOf(arr[0]).Kind() != reflect.String {
		return fmt.Errorf("Expected: [<string>, <string>, <bool>]")
	}
	c.Serial = arr[0].(string)

	if reflect.TypeOf(arr[1]).Kind() != reflect.String {
		return fmt.Errorf("Expected: [<string>, <string>, <bool>]")
	}
	c.AndroidSDKRoot = arr[1].(string)

	if reflect.TypeOf(arr[2]).Kind() != reflect.Bool {
		return fmt.Errorf("Expected: [<string>, <string>, <bool>]")
	}
	c.HasRoot = arr[2].(bool)
	return nil
}

// GetAndroidDeviceCfg retrieves Android device configuration info from the repository.
func GetAndroidDeviceCfg(workdir string) (map[string]*AndroidDeviceCfg, error) {
	r := gitiles.NewRepo("https://chromium.googlesource.com/chromium/tools/build")
	srcPath := "scripts/slave/recipe_modules/skia/android_devices.py"
	rv := map[string]*AndroidDeviceCfg{}
	if err := downloadAndExecPython(r, &rv, srcPath, workdir); err != nil {
		return nil, err
	}
	return rv, nil
}

// AndroidDeviceCfgPoller periodically reads the android_devices.py file from
// the repo using Gitiles.
func AndroidDeviceCfgPoller(workdir string) (*util.PollingStatus, error) {
	var v map[string]*AndroidDeviceCfg
	return util.NewPollingStatus(&v, func(value interface{}) error {
		cfg, err := GetAndroidDeviceCfg(workdir)
		if err != nil {
			return err
		}
		*value.(*map[string]*AndroidDeviceCfg) = cfg
		return nil
	}, 5*time.Minute)
}

// SSHDeviceCfg represents configuration information for an Android device.
type SSHDeviceCfg struct {
	User string `json:"user"`
	Host string `json:"host"`
	Port string `json:"port"`
}

func (c *SSHDeviceCfg) UnmarshalJSON(data []byte) error {
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	if len(arr) != 3 {
		return fmt.Errorf("Expected: [<string>, <string>, <string>]")
	}
	if reflect.TypeOf(arr[0]).Kind() != reflect.String {
		return fmt.Errorf("Expected: [<string>, <string>, <string>]")
	}
	c.User = arr[0].(string)

	if reflect.TypeOf(arr[1]).Kind() != reflect.String {
		return fmt.Errorf("Expected: [<string>, <string>, <string>]")
	}
	c.Host = arr[1].(string)

	if reflect.TypeOf(arr[2]).Kind() != reflect.String {
		return fmt.Errorf("Expected: [<string>, <string>, <string>]")
	}
	c.Port = arr[2].(string)
	return nil
}

// GetSSHDeviceCfg retrieves Android device configuration info from the repository.
func GetSSHDeviceCfg(workdir string) (map[string]*SSHDeviceCfg, error) {
	r := gitiles.NewRepo("https://chromium.googlesource.com/chromium/tools/build")
	srcPath := "scripts/slave/recipe_modules/skia/ssh_devices.py"
	rv := map[string]*SSHDeviceCfg{}
	if err := downloadAndExecPython(r, &rv, srcPath, workdir); err != nil {
		return nil, err
	}
	return rv, nil
}

// SSHDeviceCfgPoller periodically reads the android_devices.py file from
// the repo using Gitiles.
func SSHDeviceCfgPoller(workdir string) (*util.PollingStatus, error) {
	var v map[string]*SSHDeviceCfg
	return util.NewPollingStatus(&v, func(value interface{}) error {
		cfg, err := GetSSHDeviceCfg(workdir)
		if err != nil {
			return err
		}
		*value.(*map[string]*SSHDeviceCfg) = cfg
		return nil
	}, 5*time.Minute)
}
