package run_benchmark

type lacrosTelemetryTest struct {
	target string
	telemetryTest
}

func NewLacrosTest(target, benchmark, browser, commit, story, storyTags string) *lacrosTelemetryTest {
	return &lacrosTelemetryTest{
		target: target,
		telemetryTest: telemetryTest{
			benchmark: benchmark,
			browser:   browser,
			commit:    commit,
			story:     story,
			storyTags: storyTags,
		},
	}
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

	cmd = append(cmd, t.GetTelemetryExtraArgs()...)

	return cmd
}
