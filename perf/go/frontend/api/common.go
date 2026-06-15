package api

import (
	"net/http"
	"regexp"

	"go.skia.org/infra/perf/go/config"
)

var (
	reNonProdEnv = regexp.MustCompile(`(-autopush|-lts|-qa|-staging)(\.corp\.goog|\.luci\.app)$`)
	reLuciProd   = regexp.MustCompile(`^(https?://)?perf(-autopush)?\.luci\.app$`)
)

// getOverrideNonProdHost removes the specified suffixes from the host string if they are followed by .*.goog or .*.app.
// This is to ensure that requests from different non-prod environments (autopush, lts, qa, staging) are routed to the main environment.
func getOverrideNonProdHost(host string) string {
	host = reNonProdEnv.ReplaceAllString(host, "$2")

	// go/public-as-subset-of-internal need to fetch internal alerts for further filtering
	if reLuciProd.MatchString(host) {
		return reLuciProd.ReplaceAllString(host, "${1}chrome-perf.corp.goog")
	}
	return host
}

func preferLegacy(r *http.Request) bool {
	if config.Config.SwitchBetweenAnomalySources &&
		config.Config.FetchAnomaliesFromSql &&
		config.Config.FetchChromePerfAnomalies {
		cookie, err := r.Cookie("fetch_anomalies_from_sql")
		if err == nil {
			return cookie.Value != "true"
		}
	}
	return !config.Config.FetchAnomaliesFromSql
}

func showOnlyPublicTraces() bool {
	return config.Config.VisibilityConfig != nil && config.Config.VisibilityConfig.ShowOnlyPublicTraces
}
