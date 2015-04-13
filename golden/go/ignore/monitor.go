package ignore

import (
	"fmt"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	imetrics "go.skia.org/infra/go/metrics"
)

func oneStep(store IgnoreStore, metric metrics.Gauge) error {
	list, err := store.List()
	if err != nil {
		return err
	}
	n := 0
	for _, rule := range list {
		if time.Now().After(rule.Expires) {
			n += 1
		}
	}
	metric.Update(int64(n))
	return nil
}

// StartMonitoring starts a new monitoring routine for the given
// ignore store that counts expired ignore rules and pushes
// that info into a metric.
func Init(store IgnoreStore) error {
	numExpired := metrics.NewRegisteredGauge("num-expired-ignore-rules", metrics.DefaultRegistry)
	liveness := imetrics.NewLiveness("expired-ignore-rules-monitoring")

	err := oneStep(store, numExpired)
	if err != nil {
		return fmt.Errorf("Unable to start monitoring ignore rules: %s", err)
	}
	go func() {
		for _ = range time.Tick(time.Minute) {
			err = oneStep(store, numExpired)
			if err != nil {
				glog.Errorf("Failed one step of monitoring ignore rules: %s", err)
				continue
			}
			liveness.Update()
		}
	}()

	return nil
}
