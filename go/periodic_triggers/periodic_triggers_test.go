package periodic_triggers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	metrics_testutils "go.skia.org/infra/go/metrics2/testutils"
	"go.skia.org/infra/go/testutils"
)

func TestTriggers(t *testing.T) {
	testutils.SmallTest(t)
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	p, err := NewTriggerer(wd)
	assert.NoError(t, err)

	// Add a periodic trigger.
	ran := false
	p.Register("test", func() error {
		ran = true
		return nil
	})

	// Run periodic triggers. The trigger file does not exist, so we
	// shouldn't run the function.
	assert.False(t, ran)
	assert.NoError(t, p.RunPeriodicTriggers())
	assert.False(t, ran)

	// Write the trigger file. Cycle, ensure that the trigger file was
	// removed and the periodic task was added.
	triggerFile := path.Join(p.workdir, TRIGGER_DIRNAME, "test")
	assert.NoError(t, ioutil.WriteFile(triggerFile, []byte{}, os.ModePerm))
	assert.NoError(t, p.RunPeriodicTriggers())
	assert.True(t, ran)
	_, err = os.Stat(triggerFile)
	assert.True(t, os.IsNotExist(err))
}

func TestMetrics(t *testing.T) {
	testutils.LargeTest(t) // Calls time.Sleep()
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	metrics_testutils.Init()

	p, err := NewTriggerer(wd)
	assert.NoError(t, err)
	p.Register("test", func() error {
		return nil
	})

	// Metrics don't appear until the trigger has occurred once.
	msmt := fmt.Sprintf("liveness_%s_s", PERIODIC_TRIGGER_MEASUREMENT)
	tags := map[string]string{
		"name":    PERIODIC_TRIGGER_MEASUREMENT,
		"trigger": "test",
		"type":    "liveness",
	}
	tagsStr := metrics_testutils.StringifyTags(tags)
	assert.Equal(t, fmt.Sprintf("Could not find anything for %s%s", msmt, tagsStr), metrics_testutils.GetRecordedMetric(t, msmt, tags))

	// Trigger once, verify that metrics showed up.
	triggerFile := path.Join(p.workdir, TRIGGER_DIRNAME, "test")
	assert.NoError(t, ioutil.WriteFile(triggerFile, []byte{}, os.ModePerm))
	assert.NoError(t, p.RunPeriodicTriggers())
	v := metrics_testutils.GetRecordedMetric(t, msmt, tags)
	assert.False(t, strings.Contains(v, "Could not find"))
}
