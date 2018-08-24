/*
	Common initialization for master scripts.
*/

package master_common

import (
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"

	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	ctfeHost        = flag.String("ctfe_host", "https://ct.skia.org/", "The CTFE frontend URL.")
	ctfeInternalURL = flag.String("ctfe_internal_url", "http://ctfe:8010/", "The CTFE internal URL. Accessible from within the same cloud project.")
	Local           = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

	EmailClientSecretFile = flag.String("email_client_secret_file", "/etc/ct-email-secrets/client_secret.json", "OAuth client secret JSON file for sending email.")
	EmailTokenCacheFile   = flag.String("email_token_cache_file", "/etc/ct-email-secrets/client_token.json", "OAuth token cache file for sending email.")
	ServiceAccountFile    = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")
)

func Init(appName string) {
	common.InitWithMust(appName)
	initRest()
}

func InitWithMetrics2(appName string, promPort *string) {
	common.InitWithMust(appName, common.PrometheusOpt(promPort))
	initRest()
}

type ClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type Installed struct {
	Installed ClientConfig `json:"installed"`
}

func initRest() {
	frontend.MustInit(*ctfeHost, *ctfeInternalURL)
	if *Local {
		util.SetVarsForLocal()
	} else {
		// Initialize mailing library.
		var cfg Installed
		err := skutil.WithReadFile(*EmailClientSecretFile, func(f io.Reader) error {
			return json.NewDecoder(f).Decode(&cfg)
		})
		if err != nil {
			sklog.Fatalf("Failed to read client secrets from %q: %s", *EmailClientSecretFile, err)
		}
		// Create a copy of the token cache file since mounted secrets are read-only
		// and the access token will need to be updated for the oauth2 flow.
		fout, err := ioutil.TempFile("", "")
		if err != nil {
			sklog.Fatalf("Unable to create temp file %q: %s", fout.Name(), err)
		}
		err = skutil.WithReadFile(*EmailTokenCacheFile, func(fin io.Reader) error {
			_, err := io.Copy(fout, fin)
			if err != nil {
				err = fout.Close()
			}
			return err
		})
		if err != nil {
			sklog.Fatalf("Failed to write token cache file from %q to %q: %s", *EmailTokenCacheFile, fout.Name(), err)
		}
		*EmailTokenCacheFile = fout.Name()
		util.MailInit(cfg.Installed.ClientID, cfg.Installed.ClientSecret, *EmailTokenCacheFile)
	}
}
