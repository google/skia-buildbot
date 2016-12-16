package sklog_init

import (
	"fmt"
	"net/http"
	"os"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

// InitCloudLogging initializes the module-level logger. If an error is returned, cloud
// logging will not be used, instead glog will.
func InitCloudLogging(c *http.Client, appName, logName string) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("Could not get hostname: %s", err)
	}
	// Initialize all severity counters to 0, otherwise uncommon logs (like Error), won't
	// be in InfluxDB at all.
	initSeverities := []string{sklog.INFO, sklog.WARNING, sklog.ERROR}
	for _, severity := range initSeverities {
		metrics2.GetCounter("num_log_lines", map[string]string{"level": severity, "log_source": logName}).Reset()
	}

	var metricsCallback = func(severity string) {
		metrics2.GetCounter("num_log_lines", map[string]string{"level": severity, "log_source": logName}).Inc(1)
	}
	return sklog.InitCloudLogging(c, hostname, logName, metricsCallback)
}
