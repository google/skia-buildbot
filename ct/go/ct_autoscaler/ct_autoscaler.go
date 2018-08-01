package ct_autoscaler

import (
	"fmt"
	"sync"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/gce/ct/instance_types"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"

	"go.skia.org/infra/ct/go/master_scripts/master_common"
)

const (
	MIN_CT_INSTANCE_NUM = 1
	MAX_CT_INSTANCE_NUM = 200
)

// Interface useful for mocking.
type ICTAutoscaler interface {
	RegisterGCETask(taskId string) error
	UnregisterGCETask(taskId string) error
}

// The CTAutoscaler is a CT friendly wrapper around autoscaler.Autoscaller
// It does the following:
// * Automatically brings up all CT GCE instances when a GCE task is registered
//   and no other GCE tasks are running.
// * Automatically brings down all CT GCE instances when there are no registered
//   GCE tasks.
type CTAutoscaler struct {
	a              autoscaler.IAutoscaler
	s              swarming.ApiClient
	activeGCETasks int
	mtx            sync.Mutex
	upGauge        metrics2.Int64Metric
}

// NewCTAutoscaler returns a CT Autoscaler instance.
func NewCTAutoscaler() (*CTAutoscaler, error) {
	// Authenticated HTTP client.
	scopes := append(util.CopyStringSlice(gce.AUTH_SCOPES), swarming.AUTH_SCOPE)
	httpClient, err := auth.NewClient(*master_common.Local, "google_storage_token.data", scopes...)
	if err != nil {
		return nil, err
	}
	// Instantiate the GCE scaler.
	// TODO(borenet): Can we use go/swarming/autoscaler.Autoscaler?
	instances := autoscaler.GetInstanceRange(MIN_CT_INSTANCE_NUM, MAX_CT_INSTANCE_NUM, instance_types.CTInstance)
	a, err := autoscaler.NewAutoscaler(gce.PROJECT_ID_CT_SWARMING, gce.ZONE_CT, httpClient, instances)
	if err != nil {
		return nil, fmt.Errorf("Could not instantiate Autoscaler: %s", err)
	}

	// Instantiate the swarming client.
	s, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER_PRIVATE)
	if err != nil {
		return nil, fmt.Errorf("Could not instantiate swarming client: %s", err)
	}

	// The following metric will be set to 1 when prometheus should alert on
	// missing CT GCE bots and 0 otherwise.
	upGauge := metrics2.GetInt64Metric("ct_gce_bots_up")

	// Start from a clean slate by bringing down all CT instances since
	// activeGCETasks is initially 0. Also delete them from swarming.
	upGauge.Update(0)
	if err := a.StopAllInstances(); err != nil {
		return nil, err
	}
	// Uncomment when https://bugs.chromium.org/p/skia/issues/detail?id=7900#c7
	// is resolved.
	//if err := s.DeleteBots(a.GetNamesOfManagedInstances()); err != nil {
	//	return nil, err
	//}

	return &CTAutoscaler{a: a, s: s, upGauge: upGauge}, nil
}

func (c *CTAutoscaler) RegisterGCETask(taskId string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	sklog.Debugf("Currently have %d active GCE tasks.", c.activeGCETasks)
	sklog.Debugf("Registering task %s in CTAutoscaler.", taskId)
	if err := c.logRunningGCEInstances(); err != nil {
		return err
	}

	c.activeGCETasks += 1
	if c.activeGCETasks == 1 {
		sklog.Debugf("Starting all CT GCE instances...")
		if err := c.a.StartAllInstances(); err != nil {
			return err
		}
		if c.upGauge != nil {
			c.upGauge.Update(1)
		}
		if err := c.logRunningGCEInstances(); err != nil {
			return err
		}
	}
	return nil
}

func (c *CTAutoscaler) UnregisterGCETask(taskId string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	sklog.Debugf("Currently have %d active GCE tasks.", c.activeGCETasks)
	sklog.Debugf("Unregistering task %s in CTAutoscaler.", taskId)
	if err := c.logRunningGCEInstances(); err != nil {
		return err
	}

	c.activeGCETasks -= 1
	if c.activeGCETasks == 0 {
		sklog.Info("Stopping all CT GCE instances...")
		if c.upGauge != nil {
			c.upGauge.Update(0)
		}
		if err := c.a.StopAllInstances(); err != nil {
			return err
		}
		if err := c.logRunningGCEInstances(); err != nil {
			return err
		}

		// Delete all CT GCE instances from swarming.
		// Uncomment when https://bugs.chromium.org/p/skia/issues/detail?id=7900#c7
		// is resolved.
		//if err := c.s.DeleteBots(c.a.GetNamesOfManagedInstances()); err != nil {
		//	return err
		//}
	}
	return nil
}

func (c *CTAutoscaler) logRunningGCEInstances() error {
	if err := c.a.Update(); err != nil {
		return err
	}
	instances := c.a.GetOnlineInstances()
	sklog.Debugf("Running CT GCE instances: %s", instances)
	return nil
}
