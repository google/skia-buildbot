// pod-reaper is an application that deletes pods as directed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
)

var (
	// Flags.
	repeatInterval  = flag.Duration("repeat_interval", 15*time.Second, "How often to check for pods to reap.")
	promPort        = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	podNameEndpoint = flag.String("pod_name_endpoint", "", "A URL that returns a list pods to delete.")
)

var (
	zero = int64(0) // Needed by metav1.DeleteOptions.
)

func main() {
	common.InitWithMust("podreaper", common.PrometheusOpt(promPort))
	ctx := context.Background()

	config, err := rest.InClusterConfig()
	if err != nil {
		sklog.Fatalf("Failed to get in-cluster config: %s", err)
	}
	sklog.Infof("Auth username: %s", config.Username)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		sklog.Fatalf("Failed to get in-cluster clientset: %s", err)
	}

	client := httputils.DefaultClientConfig().With2xxOnly().Client()
	liveness := metrics2.NewLiveness("podreaper_update")
	go util.RepeatCtx(ctx, *repeatInterval, func(ctx context.Context) {
		resp, err := client.Get(*podNameEndpoint)
		if err != nil {
			sklog.Errorf("Failed to retrieve list of pods from %q: %s", *podNameEndpoint, err)
			return
		}
		var pods machine.Pods
		if err := json.NewDecoder(resp.Body).Decode(&pods); err != nil {
			sklog.Errorf("Failed to decode list of pods: %s", err)
			return
		}
		for _, podName := range pods.Names {
			if err := clientset.CoreV1().Pods("default").Delete(ctx, podName, metav1.DeleteOptions{
				GracePeriodSeconds: &zero,
			}); err != nil {
				sklog.Errorf("Failed to delete pod: %s", err)
			}
		}
		liveness.Reset()
	})
}
