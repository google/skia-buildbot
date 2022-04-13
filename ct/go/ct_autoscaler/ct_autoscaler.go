package ct_autoscaler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/autoscaler"
	"go.skia.org/infra/go/gce/ct/instance_types"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
)

const (
	MIN_CT_INSTANCE_NUM = 1
	MAX_CT_INSTANCE_NUM = 500
)

// Interface useful for mocking.
type ICTAutoscaler interface {
	RegisterGCETask(taskId string)
}

// The CTAutoscaler is a CT friendly wrapper around autoscaler.Autoscaller
// It does the following:
// * Automatically brings up all CT GCE instances when a GCE task is registered
//   and no other GCE tasks are running.
// * Automatically brings down all CT GCE instances when there are no registered
//   GCE tasks.
type CTAutoscaler struct {
	a                autoscaler.IAutoscaler
	s                swarming.ApiClient
	ctx              context.Context
	mtx              sync.Mutex
	getGCETasksCount func(ctx context.Context) (int, error)
	botsUp           bool
}

// NewCTAutoscaler returns a CT Autoscaler instance.
// getGCETasksCount refers to a function that returns the number of GCE CT tasks
// (not swarming tasks).
func NewCTAutoscaler(ctx context.Context, local bool, getGCETasksCount func(ctx context.Context) (int, error)) (*CTAutoscaler, error) {
	// Authenticated HTTP client.
	scopes := append(util.CopyStringSlice(gce.AUTH_SCOPES), swarming.AUTH_SCOPE)
	ts, err := google.DefaultTokenSource(ctx, scopes...)
	if err != nil {
		return nil, fmt.Errorf("Problem setting up default token source: %s", err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Instantiate the GCE scaler.
	instances := autoscaler.GetInstanceRange(MIN_CT_INSTANCE_NUM, MAX_CT_INSTANCE_NUM, instance_types.CTWorkerInstance)
	a, err := autoscaler.NewAutoscaler(gce.PROJECT_ID_CT_SWARMING, gce.ZONE_CT, ts, instances)
	if err != nil {
		return nil, fmt.Errorf("Could not instantiate Autoscaler: %s", err)
	}

	// Instantiate the swarming client.
	s, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER_PRIVATE)
	if err != nil {
		return nil, fmt.Errorf("Could not instantiate swarming client: %s", err)
	}

	// Get the running GCE tasks count.
	runningGCETasksCount, err := getGCETasksCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("Could not getGCETasksCount: %s", err)
	}

	c := &CTAutoscaler{
		a:                a,
		s:                s,
		ctx:              ctx,
		getGCETasksCount: getGCETasksCount,
		botsUp:           runningGCETasksCount != 0,
	}

	// Start a goroutine that watches to see if the number of running GCE tasks
	// goes to 0 and bots should be autoscaled down.
	cleanup.Repeat(2*time.Minute, func(ctx context.Context) {
		if err := c.maybeScaleDown(ctx); err != nil {
			sklog.Error(err)
		}
	}, nil)

	return c, nil
}

func (c *CTAutoscaler) maybeScaleDown(ctx context.Context) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	// Get the running GCE tasks count.
	runningGCETasksCount, err := c.getGCETasksCount(c.ctx)
	if err != nil {
		return fmt.Errorf("Could not getGCETasksCount: %s", err)
	}

	if runningGCETasksCount == 0 && c.botsUp {
		sklog.Info("Stopping all CT GCE instances...")
		if err := c.a.StopAllInstances(); err != nil {
			sklog.Errorf("Could not stop all instances: %s", err)
		}
		if err := c.logRunningGCEInstances(); err != nil {
			sklog.Errorf("Could not log running instances: %s", err)
		}

		if err := c.s.DeleteBots(ctx, c.a.GetNamesOfManagedInstances()); err != nil {
			sklog.Errorf("Could not delete all bots: %s", err)
		}
		c.botsUp = false
	}
	return nil
}

func (c *CTAutoscaler) RegisterGCETask(taskId string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	sklog.Debugf("Registering task %s in CTAutoscaler.", taskId)
	if err := c.logRunningGCEInstances(); err != nil {
		sklog.Errorf("Could not log running instances: %s", err)
	}

	if !c.botsUp {
		sklog.Debugf("Starting all CT GCE instances...")
		if err := c.a.StartAllInstances(); err != nil {
			sklog.Errorf("Could not start all instances: %s", err)
		}
		if err := c.logRunningGCEInstances(); err != nil {
			sklog.Errorf("Could not log running instances: %s", err)
		}
	}
	c.botsUp = true
}

func (c *CTAutoscaler) logRunningGCEInstances() error {
	if err := c.a.Update(); err != nil {
		return err
	}
	instances := c.a.GetOnlineInstances()
	sklog.Debugf("Running CT GCE instances: %s", instances)
	return nil
}
