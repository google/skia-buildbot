package run_benchmark

type lacrosTelemetryTest struct {
	target string
}

// GetCommand generates the command needed to execute Lacros telemetry benchmark tests
func (t *lacrosTelemetryTest) GetCommand() []string {
	cmd := []string{
		"luci-auth",
		"context",
		"--",
		"vpython3",
		"bin/run_" + t.target,
		"--remote=variable_chromeos_device_hostname",
	}

	return cmd
}
