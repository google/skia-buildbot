package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
)

// Docker registries for which AutoUpdateConfigFileAuth will write credentials.
var autoUpdateConfigFileAuthRegistries = []string{"gcr.io"}

// AutoUpdateConfigFileAuth continouously updates a Docker config file with user
// credentials in a goroutine which deletes the file and exits when the
// passed-in context expires. Returns the path to the Docker config file or any
// error which occurs.
func AutoUpdateConfigFileAuth(ctx context.Context) (string, error) {
	ts, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	tmp, err := os.MkdirTemp("", "docker-auth-")
	if err != nil {
		return "", skerr.Wrap(err)
	}
	configFilePath := filepath.Join(tmp, "config.json")

	updateConfigFile := func() (time.Duration, error) {
		tok, err := ts.Token()
		if err != nil {
			return 0, skerr.Wrap(err)
		}
		tokB64 := base64.StdEncoding.EncodeToString([]byte("oauth2accesstoken:" + tok.AccessToken))
		configFile := dockerConfigFile{
			Auths: map[string]dockerConfigFileAuthEntry{},
		}
		for _, registry := range autoUpdateConfigFileAuthRegistries {
			configFile.Auths[registry] = dockerConfigFileAuthEntry{
				Auth: tokB64,
			}
		}
		if err := util.WithWriteFile(configFilePath, func(w io.Writer) error {
			return json.NewEncoder(w).Encode(configFile)
		}); err != nil {
			return 0, skerr.Wrap(err)
		}
		if err := os.Chmod(configFilePath, 0744); err != nil {
			return 0, skerr.Wrap(err)
		}
		refreshAfter := time.Until(tok.Expiry) - time.Minute
		if refreshAfter < 0 {
			refreshAfter = time.Minute
		}
		return refreshAfter, nil
	}

	refreshAfter, err := updateConfigFile()
	if err != nil {
		util.RemoveAll(tmp)
		return "", skerr.Wrap(err)
	}
	go func() {
		for {
			select {
			case <-time.After(refreshAfter):
				refreshAfter, err = updateConfigFile()
				if err != nil {
					sklog.Errorf("Failed to update Docker config file: %s", err)
				}
			case <-ctx.Done():
				util.RemoveAll(tmp)
				return
			}
		}
	}()
	return configFilePath, nil
}

type dockerConfigFile struct {
	Auths map[string]dockerConfigFileAuthEntry `json:"auths"`
}

type dockerConfigFileAuthEntry struct {
	Auth string `json:"auth"`
}
