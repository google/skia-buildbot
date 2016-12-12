package capacity

import (
	"fmt"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/influxdb"
)

type CapacityClient struct {
	iclient          *influxdb.Client
	lastMeasurements []DimensionSet
}

func New(c *influxdb.Client) *CapacityClient {
	return &CapacityClient{iclient: c}
}

type DimensionSet struct {
	Gpu        string `json:"gpu"`
	DeviceType string `json:"device_type"`
	Device     string `json:"device"`
	Os         string `json:"os"`

	MsPerCommit    float64 `json:"ms_per_commit"`
	TasksPerCommit int     `json:"tasks_per_commit"`
	NumBots        int     `json:"num_bots"`
}

func (c *CapacityClient) QueryAll() error {
	glog.Infoln("Recounting Capacity Stats")

	botDimensions, err := c.enumerateBotDimensions()
	if err != nil {
		return fmt.Errorf("Problem enumerating bot types: %s", err)
	}
	for _, bd := range botDimensions {
		totalMs, totalTasks, err := c.getTime(bd)
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

func (c *CapacityClient) getTime(bd DimensionSet) (float64, int, error) {
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
