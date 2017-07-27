package ct_autoscaler

import (
	"fmt"
	"sync"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/gce/ct/instance_types"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/ct/go/util"
)

const (
	MIN_CT_INSTANCE_NUM = 1
	MAX_CT_INSTANCE_NUM = 200
)

// The CTAutoscaler is a CT friendly wrapper around autoscaler.Autoscaller
// It does the following:
// * Automatically brings up all CT GCE instances when a GCE task is registered
//   and no other GCE tasks are running.
// * Automatically brings down all CT GCE instances when there are no registered
//   GCE tasks.
type CTAutoscaler struct {
	a              autoscaler.IAutoscaler
	activeGCETasks int
	mtx            sync.Mutex
}

// NewCTAutoscaler returns a CT Autoscaler instance.
func NewCTAutoscaler() (*CTAutoscaler, error) {
	a, err := autoscaler.NewAutoscaler(gce.ZONE_CT, util.StorageDir, MIN_CT_INSTANCE_NUM, MAX_CT_INSTANCE_NUM, instance_types.CTInstance)
	if err != nil {
		return nil, fmt.Errorf("Could not instantiate Autoscaler: %s", err)
	}
	return &CTAutoscaler{a: a}, nil
}

func (c *CTAutoscaler) RegisterGCETask(taskId string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	sklog.Infof("Currently have %d active GCE tasks.", c.activeGCETasks)
	sklog.Infof("Registering task %s in CTAutoscaler.", taskId)
	if err := c.logRunningGCEInstances(); err != nil {
		return err
	}

	c.activeGCETasks += 1
	if c.activeGCETasks == 1 {
		sklog.Info("Starting all CT GCE instances...")
		if err := c.a.StartAllInstances(); err != nil {
			return err
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

	sklog.Infof("Currently have %d active GCE tasks.", c.activeGCETasks)
	sklog.Infof("Registering task %s in CTAutoscaler.", taskId)
	if err := c.logRunningGCEInstances(); err != nil {
		return err
	}

	c.activeGCETasks -= 1
	if c.activeGCETasks == 0 {
		sklog.Info("Stopping all CT GCE instances...")
		if err := c.a.StopAllInstances(); err != nil {
			return err
		}
		if err := c.logRunningGCEInstances(); err != nil {
			return err
		}
	}
	return nil
}

func (c *CTAutoscaler) logRunningGCEInstances() error {
	instances, err := c.a.GetRunningInstances()
	if err != nil {
		return err
	}
	sklog.Infof("Running CT GCE instances: %s", instances)
	return nil
}
