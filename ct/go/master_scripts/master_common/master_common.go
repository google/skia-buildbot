/*
	Common initialization for master scripts.
*/

package master_common

import (
	"flag"

	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

var (
	ctfeURL         = flag.String("ctfe_url", "https://ct.skia.org/", "The CTFE frontend URL.")
	ctfeInternalURL = flag.String("ctfe_internal_url", "http://ctfe:9000/", "The CTFE internal URL. Accessible from within the same cloud project.")
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
	common.InitWithMust(
		appName,
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	initRest()
}

func initRest() {
	frontend.MustInit(*ctfeURL, *ctfeInternalURL)
	if *Local {
		util.SetVarsForLocal()
	} else {
		// Initialize mailing library.
		if err := util.MailInit(*EmailClientSecretFile, *EmailTokenCacheFile); err != nil {
			sklog.Fatalf("Could not initialize mailing library: %s", err)
		}
	}
}
