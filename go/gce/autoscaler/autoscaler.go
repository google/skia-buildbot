package autoscaler

import (
	"fmt"
	"sort"
	"sync"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
)

// Interface useful for mocking.
// TODO(borenet): This doesn't really "auto" scale anything.
type IAutoscaler interface {
	// GetInstanceStatuses returns a map of instance names to booleans
	// indicating whether each instance is online as of the last Update().
	GetInstanceStatuses() map[string]bool

	// GetNamesOfManagedInstances returns names of all instances managed by
	// this autoscaler.
	GetNamesOfManagedInstances() []string

	// GetOnlineInstances returns a slice of all online instance names as of
	// the last Update().
	GetOnlineInstances() []string

	// Start the given instances.
	// Note: This method returns when all instances are in RUNNING state.
	// Does not check to see if they are ready (ssh-able).
	Start([]string) error

	// StartAllInstances starts all instances.
	// Note: This method returns when all instances are in RUNNING state.
	// Does not check to see if they are ready (ssh-able).
	StartAllInstances() error

	// Stop the given instances.
	Stop([]string) error

	// StopAllInstances stops all instances.
	StopAllInstances() error

	// Update determines which instances are online and caches that
	// information.
	Update() error
}

// Autoscaler is a struct used for autoscaling instances in GCE.
type Autoscaler struct {
	g             *gce.GCloud
	instanceNames []string
	instances     map[string]*gce.Instance
	mtx           sync.RWMutex // protects a.online
	online        []bool
}

// Helper function for creating lists of instances. The given range is
// inclusive.
func GetInstanceRange(min, max int, getInstance func(int) *gce.Instance) []*gce.Instance {
	rv := make([]*gce.Instance, 0, max-min+1)
	for i := min; i <= max; i++ {
		rv = append(rv, getInstance(i))
	}
	return rv
}

// Helper function for creating lists of instances.
func GetInstanceSet(intSet string, getInstance func(int) *gce.Instance) ([]*gce.Instance, error) {
	nums, err := util.ParseIntSet(intSet)
	if err != nil {
		return nil, err
	}
	rv := make([]*gce.Instance, 0, len(nums))
	for _, num := range nums {
		rv = append(rv, getInstance(num))
	}
	return rv, nil
}

// NewAutoscaler returns an Autoscaler instance which manages the given GCE
// instances. Automatically calls Update().
func NewAutoscaler(projectId, zone string, ts oauth2.TokenSource, instances []*gce.Instance) (*Autoscaler, error) {
	// Create the GCloud object.
	g, err := gce.NewGCloud(projectId, zone, ts)
	if err != nil {
		return nil, err
	}
	// Create map of instances.
	instanceMap := make(map[string]*gce.Instance, len(instances))
	instanceNames := make([]string, 0, len(instances))
	for _, instance := range instances {
		instanceMap[instance.Name] = instance
		instanceNames = append(instanceNames, instance.Name)
	}
	sort.Strings(instanceNames)
	a := &Autoscaler{
		g:             g,
		instanceNames: instanceNames,
		instances:     instanceMap,
	}
	if err := a.Update(); err != nil {
		return nil, err
	}
	return a, nil
}

// See documentation for IAutoscaler.
func (a *Autoscaler) Update() error {
	online := make([]bool, len(a.instanceNames))
	group := util.NewNamedErrGroup()
	for idx, instance := range a.instanceNames {
		// https://golang.org/doc/faq#closures_and_goroutines
		idx := idx
		instance := instance
		group.Go(instance, func() error {
			isOnline, err := a.g.IsInstanceRunning(instance)
			if err != nil {
				return err
			}
			online[idx] = isOnline
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.online = online
	return nil
}

// See documentation for IAutoscaler.
func (a *Autoscaler) GetInstanceStatuses() map[string]bool {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	rv := make(map[string]bool, len(a.instanceNames))
	for idx, name := range a.instanceNames {
		rv[name] = a.online[idx]
	}
	return rv
}

// See documentation for IAutoscaler.
func (a *Autoscaler) GetOnlineInstances() []string {
	a.mtx.RLock()
	defer a.mtx.RUnlock()
	onlineInstances := make([]string, 0, len(a.instances))
	for idx, name := range a.instanceNames {
		if a.online[idx] {
			onlineInstances = append(onlineInstances, name)
		}
	}
	return onlineInstances
}

// See documentation for IAutoscaler.
func (a *Autoscaler) GetNamesOfManagedInstances() []string {
	return util.CopyStringSlice(a.instanceNames)
}

// Run the given function in parallel over the given instances.
func (a *Autoscaler) processInstances(instanceNames []string, fn func(*gce.Instance) error) error {
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
			return fn(instance)
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return nil
}

// See documentation for IAutoscaler.
func (a *Autoscaler) Start(instanceNames []string) error {
	return a.processInstances(instanceNames, func(instance *gce.Instance) error {
		return a.g.StartWithoutReadyCheck(instance)
	})
}

// See documentation for IAutoscaler.
func (a *Autoscaler) StartAllInstances() error {
	return a.Start(a.instanceNames)
}

// See documentation for IAutoscaler.
func (a *Autoscaler) Stop(instanceNames []string) error {
	return a.processInstances(instanceNames, func(instance *gce.Instance) error {
		return a.g.Stop(instance)
	})
}

// See documentation for IAutoscaler.
func (a *Autoscaler) StopAllInstances() error {
	return a.Stop(a.instanceNames)
}

var _ IAutoscaler = &Autoscaler{}
