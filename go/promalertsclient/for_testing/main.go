package main

import (
	"flag"
	"net/http"

	"github.com/prometheus/alertmanager/dispatch"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/promalertsclient"
	"go.skia.org/infra/go/sklog"
)

var alertsEndpoint = flag.String("alerts_endpoint", "skia-prom:8001", "The Prometheus GCE name and port")

func main() {
	common.Init()

	ac := promalertsclient.New(&http.Client{}, *alertsEndpoint)
	bots, err := ac.GetAlerts(func(a dispatch.APIAlert) bool {
		alertName := string(a.Labels["alertname"])
		return alertName == "BotMissing" || alertName == "BotQuarantined"
	})
	sklog.Errorf("Error %v", err)
	sklog.Infof("Bots %v", bots)
}
