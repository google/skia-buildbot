package ignore

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

func oneStep(store IgnoreStore, metric metrics2.Int64Metric) error {
	list, err := store.List(false)
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
	numExpired := metrics2.GetInt64Metric("gold_num_expired_ignore_rules", nil)
	liveness := metrics2.NewLiveness("gold_expired_ignore_rules_monitoring")

	err := oneStep(store, numExpired)
	if err != nil {
		return fmt.Errorf("Unable to start monitoring ignore rules: %s", err)
	}
	go func() {
		for range time.Tick(time.Minute) {
			err = oneStep(store, numExpired)
			if err != nil {
				sklog.Errorf("Failed one step of monitoring ignore rules: %s", err)
				continue
			}
			liveness.Reset()
		}
	}()

	return nil
}
