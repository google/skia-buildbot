package influxdb_init

/*
	influxdb_init provides an entry point for InfluxDB using metadata which
	is outside of the main influxdb package. This prevents circular imports
	between influxdb, metadata, and http.
*/

import (
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/metadata"
)

// NewClientFromParamsAndMetadata returns a Client with credentials obtained
// from a combination of the given parameters and metadata, depending on whether
// the program is running in local mode.
func NewClientFromParamsAndMetadata(host, user, password, database string, local bool) (*influxdb.Client, error) {
	if !local {
		var err error
		user, err = metadata.ProjectGet(metadata.INFLUXDB_NAME)
		if err != nil {
			return nil, err
		}
		password, err = metadata.ProjectGet(metadata.INFLUXDB_PASSWORD)
		if err != nil {
			return nil, err
		}
		database, err = metadata.ProjectGet(metadata.INFLUXDB_DATABASE)
		if err != nil {
			return nil, err
		}
		host, err = metadata.ProjectGet(metadata.INFLUXDB_HOST)
		if err != nil {
			return nil, err
		}
	}
	return influxdb.NewClient(host, user, password, database)
}
