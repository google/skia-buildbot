// +build dataonly

package run

import (
	"github.com/skia-dev/influxdb/services/admin"
	"github.com/skia-dev/influxdb/services/collectd"
	"github.com/skia-dev/influxdb/services/continuous_querier"
	"github.com/skia-dev/influxdb/services/graphite"
	"github.com/skia-dev/influxdb/services/httpd"
	"github.com/skia-dev/influxdb/services/opentsdb"
	"github.com/skia-dev/influxdb/services/precreator"
	"github.com/skia-dev/influxdb/services/retention"
	"github.com/skia-dev/influxdb/services/udp"
)

func (s *Server) appendRetentionPolicyService(c retention.Config) {
}

func (s *Server) appendAdminService(c admin.Config) {
}

func (s *Server) appendHTTPDService(c httpd.Config) {
}

func (s *Server) appendCollectdService(c collectd.Config) {
}

func (s *Server) appendOpenTSDBService(c opentsdb.Config) error {
	return nil
}

func (s *Server) appendGraphiteService(c graphite.Config) error {
	return nil
}

func (s *Server) appendPrecreatorService(c precreator.Config) error {
	return nil
}

func (s *Server) appendUDPService(c udp.Config) {
}

func (s *Server) appendContinuousQueryService(c continuous_querier.Config) {
}
