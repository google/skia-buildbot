package gce

import (
	"fmt"
	"path/filepath"

	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/util"
)

// Autoscaller is a struct used for autoscalling instances in GCE.
type Autoscaller struct {
	project      string
	g            *gce.GCloud
	workdir      string
	zone         string
	namePrefix   string
	minInstances int
	maxInstances int
	getInstance  func(int) *gce.Instance
}

// NewAutoscaller returns an Autoscaller instance.
func NewAutoscaller(zone, workdir, namePrefix string, minInstances, maxInstances int, getInstance func(int) *gce.Instance) (*Autoscaller, error) {
	// Get the absolute workdir.
	wdAbs, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}

	// Create the GCloud object.
	g, err := gce.NewGCloud(zone, wdAbs)
	if err != nil {
		return nil, err
	}
	return &Autoscaller{
		project:      gce.PROJECT_ID,
		g:            g,
		workdir:      workdir,
		zone:         zone,
		minInstances: minInstances,
		maxInstances: maxInstances,
		getInstance:  getInstance,
		namePrefix:   namePrefix,
	}, nil
}

// GetRunningInstances returns a slice of all instance names matching the zone, project and
// namePrefix.
func (a *Autoscaller) GetRunningInstances() ([]string, error) {
	instances, err := a.g.ListRunningInstanceNames(a.namePrefix)
	if err != nil {
		return nil, err
	}
	return instances, nil
}

// StopAllInstances stops all instances inclusively between [minInstances, maxInstances].
func (a *Autoscaller) StopAllInstances() error {
	instanceNums, err := util.ParseIntSet(fmt.Sprintf("%d-%d", a.minInstances, a.maxInstances))
	if err != nil {
		return err
	}

	// Perform the requested operation.
	group := util.NewNamedErrGroup()
	for _, num := range instanceNums {
		vm := a.getInstance(num)
		group.Go(vm.Name, func() error {
			return a.g.Stop(vm)
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return nil
}

// StartAllInstances starts all instances inclusively between [minInstances, maxInstances].
func (a *Autoscaller) StartAllInstances() error {
	instanceNums, err := util.ParseIntSet(fmt.Sprintf("%d-%d", a.minInstances, a.maxInstances))
	if err != nil {
		return err
	}

	// Perform the requested operation.
	group := util.NewNamedErrGroup()
	for _, num := range instanceNums {
		vm := a.getInstance(num)
		group.Go(vm.Name, func() error {
			return a.g.Start(vm)
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return nil
}
