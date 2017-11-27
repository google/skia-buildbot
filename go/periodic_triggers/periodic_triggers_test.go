package periodic_triggers

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestTriggers(t *testing.T) {
	testutils.SmallTest(t)
	ctx := context.Background()
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, wd)

	p, err := NewTriggerer(wd)
	assert.NoError(t, err)

	// Add a periodic trigger.
	ran := false
	p.Register("test", func(ctx context.Context) error {
		ran = true
		return nil
	})

	// Run periodic triggers. The trigger file does not exist, so we
	// shouldn't run the function.
	assert.False(t, ran)
	assert.NoError(t, p.RunPeriodicTriggers(ctx))
	assert.False(t, ran)

	// Write the trigger file. Cycle, ensure that the trigger file was
	// removed and the periodic task was added.
	triggerFile := path.Join(p.workdir, TRIGGER_DIRNAME, "test")
	assert.NoError(t, ioutil.WriteFile(triggerFile, []byte{}, os.ModePerm))
	assert.NoError(t, p.RunPeriodicTriggers(ctx))
	assert.True(t, ran)
	_, err = os.Stat(triggerFile)
	assert.True(t, os.IsNotExist(err))
}
