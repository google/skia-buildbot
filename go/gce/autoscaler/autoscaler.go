package autoscaler

import (
	"fmt"
	"path/filepath"
	"sync"

	compute "google.golang.org/api/compute/v0.beta"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/util"
)

// Interface useful for mocking.
// TODO(borenet): This doesn't really "auto" scale anything.
type IAutoscaler interface {
	GetInstanceStatuses() map[string]bool
	GetNamesOfManagedInstances() []string
	GetRunningInstances() []string
	Start([]string) error
	StartAllInstances() error
	Stop([]string) error
	StopAllInstances() error
	Update() error
}

// Autoscaler is a struct used for autoscaling instances in GCE.
type Autoscaler struct {
	g             *gce.GCloud
	mtx           sync.RWMutex // protects a.running
	workdir       string
	instanceNames []string
	instances     map[string]*gce.Instance
	running       []bool
}

// NewAutoscaler returns an Autoscaler instance which manages numbered GCE
// instances within the given inclusive range.
func NewAutoscaler(projectId, zone, workdir string, minInstanceNum, maxInstanceNum int, getInstance func(int) *gce.Instance) (*Autoscaler, error) {
	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}

	// Create the GCloud object.
	httpClient, err := auth.NewClient(false, "", compute.CloudPlatformScope, compute.ComputeScope, compute.DevstorageFullControlScope)
	if err != nil {
		return nil, err
	}
	g, err := gce.NewGCloudWithClient(projectId, zone, wdAbs, httpClient)
	if err != nil {
		return nil, err
	}
	// Create map of instances.
	instanceNames := make([]string, 0, maxInstanceNum-minInstanceNum+1)
	instances := make(map[string]*gce.Instance, maxInstanceNum-minInstanceNum+1)
	for num := minInstanceNum; num <= maxInstanceNum; num++ {
		instance := getInstance(num)
		instanceNames = append(instanceNames, instance.Name)
		instances[instance.Name] = instance
	}
	return &Autoscaler{
		g:             g,
		workdir:       workdir,
		instanceNames: instanceNames,
		instances:     instances,
	}, nil
}

// Update determines which instances are running and caches that information.
func (a *Autoscaler) Update() error {
	running := make([]bool, len(a.instanceNames))
	group := util.NewNamedErrGroup()
	for idx, instance := range a.instanceNames {
		// https://golang.org/doc/faq#closures_and_goroutines
		idx := idx
		instance := instance
		group.Go(instance, func() error {
			// TODO(borenet): This could fail, so it should return an error.
			running[idx] = a.g.IsInstanceRunning(instance)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.running = running
	return nil
}

// GetInstanceStatuses returns a map of instance names to booleans indicating
// whether each instance is running as of the last Update().
func (a *Autoscaler) GetInstanceStatuses() map[string]bool {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := make(map[string]bool, len(a.instanceNames))
	for idx, name := range a.instanceNames {
		rv[name] = a.running[idx]
	}
	return rv
}

// GetRunningInstances returns a slice of all running instance names as of the
// last Update().
func (a *Autoscaler) GetRunningInstances() []string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	runningInstances := make([]string, 0, len(a.instances))
	for idx, name := range a.instanceNames {
		if a.running[idx] {
			runningInstances = append(runningInstances, name)
		}
	}
	return runningInstances
}

// GetNamesOfManagedInstances returns names of all instances managed by this
// autoscaler.
func (a *Autoscaler) GetNamesOfManagedInstances() []string {
	return util.CopyStringSlice(a.instanceNames)
}

// Start the given instances.
// Note: This method returns when all instances are in RUNNING state. Does not
// check to see if they are ready (ssh-able).
func (a *Autoscaler) Start(instanceNames []string) error {
	// Get the requested instances.
	instances := make([]*gce.Instance, 0, len(instanceNames))
	for _, name := range instanceNames {
		instance, ok := a.instances[name]
		if !ok {
			return fmt.Errorf("Unknown instance %q", name)
		}
		instances = append(instances, instance)
	}
	// Start the instances.
	group := util.NewNamedErrGroup()
	for _, instance := range instances {
		instance := instance // https://golang.org/doc/faq#closures_and_goroutines
		group.Go(instance.Name, func() error {
			return a.g.StartWithoutReadyCheck(instance)
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return nil
}

// StartAllInstances starts all instances.
// Note: This method returns when all instances are in RUNNING state. Does not
// check to see if they are ready (ssh-able).
func (a *Autoscaler) StartAllInstances() error {
	return a.Start(a.instanceNames)
}

// Stop the given instances.
func (a *Autoscaler) Stop(instanceNames []string) error {
	// Get the requested instances.
	instances := make([]*gce.Instance, 0, len(instanceNames))
	for _, name := range instanceNames {
		instance, ok := a.instances[name]
		if !ok {
			return fmt.Errorf("Unknown instance %q", name)
		}
		instances = append(instances, instance)
	}
	// Stop the instances.
	group := util.NewNamedErrGroup()
	for _, instance := range instances {
		instance := instance // https://golang.org/doc/faq#closures_and_goroutines
		group.Go(instance.Name, func() error {
			return a.g.Stop(instance)
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return nil
}

// StopAllInstances stops all instances.
func (a *Autoscaler) StopAllInstances() error {
	return a.Stop(a.instanceNames)
}

var _ IAutoscaler = &Autoscaler{}
