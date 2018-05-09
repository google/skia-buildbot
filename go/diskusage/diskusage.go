package diskusage

import (
	"time"

	"github.com/ricochet2200/go-disk-usage/du"
	"go.skia.org/infra/go/metrics2"
)

const (
	PERIOD = 15 * time.Second
)

var (
	metrics = []string{"free", "available", "size"}
)

// DiskUsage gathers free and available space on a series of directories and
// publishes the values under metrics2.
type DiskUsage struct {
	disks []*diskUsageMetrics
}

func (d *DiskUsage) run() {
	for range time.Tick(PERIOD) {
		for _, disk := range d.disks {
			disk.gather()
		}
	}
}

// diskUsageMetrics gathers metrics on a single directory.
type diskUsageMetrics struct {
	du      *du.DiskUsage
	metrics map[string]metrics2.Int64Metric
}

func (d *diskUsageMetrics) gather() {
	d.metrics["free"].Update(int64(d.du.Free()))
	d.metrics["available"].Update(int64(d.du.Available()))
	d.metrics["size"].Update(int64(d.du.Size()))
}

func newDiskUsageMetrics(dir string) *diskUsageMetrics {
	ret := &diskUsageMetrics{
		du:      du.NewDiskUsage(dir),
		metrics: map[string]metrics2.Int64Metric{},
	}
	for _, name := range metrics {
		ret.metrics[name] = metrics2.GetInt64Metric("disk_"+name, map[string]string{"directory": dir})
	}
	return ret
}

func NewDiskUsage(dirs []string) *DiskUsage {
	ret := &DiskUsage{
		disks: make([]*diskUsageMetrics, 0, len(dirs)),
	}

	for _, dir := range dirs {
		ret.disks = append(ret.disks, newDiskUsageMetrics(dir))
	}

	go ret.run()
	return ret
}
