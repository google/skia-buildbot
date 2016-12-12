package capacity

// This package makes multiple queries to InfluxDB to get metrics that allow
// us to gauge theoretical capacity needs. Presently, the last 3 days worth of
// swarming data is used as the basis for these metrics.

import (
	"fmt"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/influxdb"
)

type CapacityClient struct {
	iclient *influxdb.Client
	// The cached measurements
	lastMeasurements []DimensionSet
}

func New(c *influxdb.Client) *CapacityClient {
	return &CapacityClient{iclient: c}
}

// DimensionSet represents one device configuration that we test on. It contains the
// key stats needed to gauge capacity needs.  Os will always be filled in, along
// with ONE of Gpu, DeviceType and Device.
type DimensionSet struct {
	Gpu        string `json:"gpu"`
	DeviceType string `json:"device_type"`
	Device     string `json:"device"`
	Os         string `json:"os"`

	MsPerCommit    float64 `json:"ms_per_commit"`
	TasksPerCommit int     `json:"tasks_per_commit"`
	NumBots        int     `json:"num_bots"`
}

// QueryAll updates the capacity metrics.  First it recomputes the set of all
// BotDimensions we run tasks on. Then, for each of these, it makes two queries:
// one query to count the total time needed per commit and the number of tasks per
// commit, and one query to count the number of bots with those dimensions that
// have run tasks recently.
func (c *CapacityClient) QueryAll() error {
	glog.Infoln("Recounting Capacity Stats")

	botDimensions, err := c.enumerateBotDimensions()
	if err != nil {
		return fmt.Errorf("Problem enumerating bot types: %s", err)
	}
	for _, bd := range botDimensions {
		totalMs, totalTasks, err := c.getTaskStats(bd)
		if err != nil {
			return fmt.Errorf("Could not get time for %s", err)
		}
		bd.MsPerCommit = totalMs
		bd.TasksPerCommit = totalTasks
		totalBots, err := c.getBots(bd)
		if err != nil {
			return fmt.Errorf("Could not get time for %s", err)
		}
		bd.NumBots = totalBots
		glog.Infof("Finished Dimension Set: %+v\n", bd)
	}
	c.lastMeasurements = botDimensions
	return err
}

// StartLoading begins an infinite loop to recompute the capacity metrics after a
// given period of time.  Any errors are logged, but the loop is not broken.
func (c *CapacityClient) StartLoading(period time.Duration) {
	go func() {
		if err := c.QueryAll(); err != nil {
			glog.Errorf("There was a problem counting capacity stats")
		}
		for _ = range time.Tick(period) {
			if err := c.QueryAll(); err != nil {
				glog.Errorf("There was a problem counting capacity stats")
			}
		}
	}()
}

func (c *CapacityClient) CapacityMetrics() []DimensionSet {
	return c.lastMeasurements
}

// getTaskStats makes an InfluxDB query to count the total time needed per commit and
// the number of tasks per commit. The query works by getting the average time for each
// of the tasks that runs on a bot and summing these averages together.  Then it simply
// counts the number of tasks. Two assumptions are made: that every task is unique
// by its name (this is generally a good assumption) and every task name returned is
// run on every commit. This second assumption breaks down when tasks are renamed,
// but hopefully that won't happen too often. The assumption resolves itself once
// the window has passed the renaming (currently 3 days).
func (c *CapacityClient) getTaskStats(bd DimensionSet) (float64, int, error) {
	var tasks []*influxdb.Point
	var err error
	if bd.Gpu != "" {
		tasks, err = c.iclient.Query("skmetrics", fmt.Sprintf(GPU_QUERY, bd.Gpu, bd.Os), 1)
		if err != nil {
			return 0, 0, fmt.Errorf("Could not make query: %s", err)
		}
	} else if bd.DeviceType != "" {
		tasks, err = c.iclient.Query("skmetrics", fmt.Sprintf(DEVICE_TYPE_QUERY, bd.DeviceType, bd.Os), 1)
		if err != nil {
			return 0, 0, fmt.Errorf("Could not make query: %s", err)
		}
	} else if bd.Device != "" {
		tasks, err = c.iclient.Query("skmetrics", fmt.Sprintf(DEVICE_QUERY, bd.Device, bd.Os), 1)
		if err != nil {
			return 0, 0, fmt.Errorf("Could not make query: %s", err)
		}
	}
	totalMs := 0.0
	for _, task := range tasks {
		f, err := task.Values[0].Float64()
		if err != nil {
			return totalMs, len(tasks), fmt.Errorf("Malformed result: %s", err)
		}
		totalMs += f
	}
	return totalMs, len(tasks), nil
}

// getBots makes an InfluxDB query to count the number of bots that have completed a task.
func (c *CapacityClient) getBots(bd DimensionSet) (int, error) {
	if bd.Gpu != "" {
		tasks, err := c.iclient.Query("skmetrics", fmt.Sprintf(GPU_COUNT, bd.Gpu, bd.Os), 1)
		return len(tasks), err
	} else if bd.DeviceType != "" {
		tasks, err := c.iclient.Query("skmetrics", fmt.Sprintf(DEVICE_TYPE_COUNT, bd.DeviceType, bd.Os), 1)
		return len(tasks), err
	} else if bd.Device != "" {
		tasks, err := c.iclient.Query("skmetrics", fmt.Sprintf(DEVICE_COUNT, bd.Device, bd.Os), 1)
		return len(tasks), err
	}
	return 0, nil
}

// enumerateBotDimensions makes an InfluxDB query to create the set of all DimensionSets
// that have completed tasks recently. From this query, it creates DimensonSet structs
// and returns the slice of them.
func (c *CapacityClient) enumerateBotDimensions() ([]DimensionSet, error) {
	q, err := c.iclient.Query("skmetrics", ENUMERATE_QUERY, 1)
	retVal := []DimensionSet{}
	if err != nil {
		return retVal, err
	}

	for _, r := range q {
		d := DimensionSet{
			Gpu:        r.Tags["dimension-gpu"],
			DeviceType: r.Tags["dimension-device_type"],
			Device:     r.Tags["dimension-device"],
			Os:         r.Tags["dimension-os"],
		}
		retVal = append(retVal, d)
	}
	return retVal, nil
}

const GPU_QUERY = `SELECT mean("value") FROM "swarming.tasks.duration" WHERE  "dimension-gpu" = '%s' and "dimension-os" = '%s' and time > now() - 3d GROUP BY "task-name"`

// Android devices
const DEVICE_TYPE_QUERY = `SELECT mean("value") FROM "swarming.tasks.duration" WHERE  "dimension-device_type" = '%s' and "dimension-os" = '%s' and time > now() - 3d GROUP BY "task-name"`

// Ipads
const DEVICE_QUERY = `SELECT mean("value") FROM "swarming.tasks.duration" WHERE  "dimension-device" = '%s' and "dimension-os" = '%s' and time > now() - 3d GROUP BY "task-name"`

const GPU_COUNT = `SELECT count("value") FROM "swarming.tasks.duration" WHERE  "dimension-gpu" = '%s' and "dimension-os" = '%s' and time > now() - 3d GROUP BY "bot-id"`

const DEVICE_TYPE_COUNT = `SELECT count("value") FROM "swarming.tasks.duration" WHERE  "dimension-device_type" = '%s' and "dimension-os" = '%s' and time > now() - 3d GROUP BY "bot-id"`

const DEVICE_COUNT = `SELECT count("value") FROM "swarming.tasks.duration" WHERE  "dimension-device" = '%s' and "dimension-os" = '%s' and time > now() - 3d GROUP BY "bot-id"`

const ENUMERATE_QUERY = `SELECT count("value") FROM "swarming.tasks.duration" WHERE time > now() - 1d GROUP BY "dimension-device_type", "dimension-gpu", "dimension-os", "dimension-device"`
