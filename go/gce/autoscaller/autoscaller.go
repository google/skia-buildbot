package gce

import (
	"fmt"
	//"google.golang.org/api/compute/v0.alpha"
	"go.skia.org/infra/go/gce"
	"go.skia.org/infra/go/gce/ct/instance_types"
	"path/filepath"
)

var ()

// TODO(rmistry): This should be used by the CT master to control everything via the master scripts.
// Because the master scripts know how many instances should be brought up.

// Autoscaller is a struct used for autoscalling instances in GCE.
type Autoscaller struct {
	project      string
	g            *gce.GCloud
	workdir      string
	zone         string
	minInstances int
	maxInstances int
	getInstance  func(int) *gce.Instance
	// TODO(rmistry): Will definitely need a mutex to control access to everything.
}

// NewAutoscaller returns an Autoscaller instance.
func NewAutoscaller(zone, workdir string, minInstances, maxInstances int, getInstance func(int) *gce.Instance) (*Autoscaller, error) {
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
	}, nil
}

func (a *Autoscaller) GetInstances(num int) error {
	instances, err := a.g.ListInstanceNames(instance_types.CT_WORKER_PREFIX)
	fmt.Println(instances)
	if err != nil {
		return err
	}
	return nil
}
