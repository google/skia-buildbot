/*
	Common initialization for master scripts.
*/

package master_common

import (
	"flag"
	"path/filepath"

	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
)

var (
	ctfeURL = flag.String("ctfe_url", "https://ct.skia.org/", "The CTFE frontend URL.")
	//ctfeInternalURL = flag.String("ctfe_internal_url", "http://ctfe-staging:9000/", "The CTFE internal URL. Accessible from within the same cloud project.")
	Local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

	EmailClientSecretFile = flag.String("email_client_secret_file", "/etc/ct-email-secrets/client_secret.json", "OAuth client secret JSON file for sending email.")
	EmailTokenCacheFile   = flag.String("email_token_cache_file", "/etc/ct-email-secrets/client_token.json", "OAuth token cache file for sending email.")
	// Should use token stuff for this?
	// ServiceAccountFile    = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")

	// Datastore params
	namespace   = flag.String("ds_namespace", "cluster-telemetry", "The Cloud Datastore namespace, such as 'cluster-telemetry'.")
	projectName = flag.String("ds_project_name", "skia-public", "The Google Cloud project name.")
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
	frontend.MustInit(*ctfeURL)

	// Initialize the datastore.
	if err := ds.Init(*projectName, *namespace); err != nil {
		sklog.Fatal(err)
	}
	if *Local {
		util.SetVarsForLocal()
	} else {
		// Initialize mailing library.
		util.MailInit(filepath.Join(util.StorageDir, "email.data"))
	}
}
