package tsdb

import "github.com/skia-dev/influxdb/models"

// Executor is an interface for a query executor.
type Executor interface {
	Execute(closing <-chan struct{}) <-chan *models.Row
}
