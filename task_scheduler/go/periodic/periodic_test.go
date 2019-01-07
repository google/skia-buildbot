package periodic

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/specs"
)

func setup(t *testing.T) (*Periodic, func()) {
	testutils.MediumTest(t)
	testutils.ManualTest(t)
	instance := fmt.Sprintf("test-%s", uuid.New())
	p, err := New(context.Background(), firestore.FIRESTORE_PROJECT, instance, nil)
	assert.NoError(t, err)
	cleanup := func() {
		c := p.client
		assert.NoError(t, firestore.RecursiveDelete(c, c.ParentDoc, 5, 30*time.Second))
		assert.NoError(t, p.Close())
	}
	return p, cleanup
}

func TestPeriodicSuccess(t *testing.T) {
	p, cleanup := setup(t)
	defer cleanup()

	ctx := context.Background()

	// Launch a call to MaybeTrigger in the background.
	ch1 := make(chan bool)
	ch2 := make(chan bool)
	go func() {
		assert.NoError(t, p.MaybeTrigger(ctx, specs.TRIGGER_NIGHTLY, func() error {
			// Signal that this func has started.
			ch1 <- true
			// Wait for the signal that we can return.
			<-ch2
			return nil
		}))
		// Signal that MaybeTrigger has finished.
		ch1 <- true
	}()

	// Wait for the above func to start.
	<-ch1

	// Run a second instance of MaybeTrigger. This one shouldn't run, since
	// the above func is already running.
	assert.NoError(t, p.MaybeTrigger(ctx, specs.TRIGGER_NIGHTLY, func() error {
		assert.FailNow(t, "This func shouldn't run.")
		return nil
	}))

	// Signal to the first func that it's okay to return.
	ch2 <- true

	// Wait for the first func to be done.
	<-ch1

	// Run a third instance of MaybeTrigger. This one shouldn't run, since
	// the first func completed successfully.
	assert.NoError(t, p.MaybeTrigger(ctx, specs.TRIGGER_NIGHTLY, func() error {
		assert.FailNow(t, "This func shouldn't run.")
		return nil
	}))
}

func TestPeriodicFailure(t *testing.T) {
	p, cleanup := setup(t)
	defer cleanup()

	ctx := context.Background()

	// Launch a call to MaybeTrigger in the background.
	ch1 := make(chan bool)
	ch2 := make(chan bool)
	go func() {
		assert.Error(t, p.MaybeTrigger(ctx, specs.TRIGGER_NIGHTLY, func() error {
			// Signal that this func has started.
			ch1 <- true
			// Wait for the signal that we can return.
			<-ch2
			return errors.New("fail")
		}))
		// Signal that MaybeTrigger has finished.
		ch1 <- true
	}()

	// Wait for the above func to start.
	<-ch1

	// Run a second instance of MaybeTrigger. This one shouldn't run, since
	// the above func is already running.
	assert.NoError(t, p.MaybeTrigger(ctx, specs.TRIGGER_NIGHTLY, func() error {
		assert.FailNow(t, "This func shouldn't run.")
		return nil
	}))

	// Signal to the first func that it's okay to return.
	ch2 <- true

	// Wait for the first func to be done.
	<-ch1

	// Run a third instance of MaybeTrigger. This one should run, since the
	// first func did not complete successfully.
	ran := false
	assert.NoError(t, p.MaybeTrigger(ctx, specs.TRIGGER_NIGHTLY, func() error {
		ran = true
		return nil
	}))
	assert.True(t, ran)
}
