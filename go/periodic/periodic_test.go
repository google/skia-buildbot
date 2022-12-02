package periodic

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/emulators/gcp_emulator"
)

func TestPeriodic(t *testing.T) {
	gcp_emulator.RequirePubSub(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test validation.
	assert.EqualError(t, Trigger(ctx, "bogus", uuid.New().String(), nil), "Invalid trigger name \"bogus\"")
	assert.EqualError(t, Trigger(ctx, TRIGGER_NIGHTLY, "", nil), "Invalid trigger ID \"\"")

	subName := fmt.Sprintf("periodic-test-%s", uuid.New())
	expectCh := make(chan string)
	rvCh := make(chan bool)

	assert.NoError(t, Listen(ctx, subName, nil, func(_ context.Context, trigger, id string) bool {
		expectTrigger := <-expectCh
		expectId := <-expectCh
		assert.Equal(t, expectTrigger, trigger)
		assert.Equal(t, expectId, id)
		return <-rvCh
	}))

	check := func(trigger, id string, rv bool) {
		expectCh <- trigger
		expectCh <- id
		rvCh <- rv
	}
	triggerAndCheck := func(trigger, id string, rv bool) {
		assert.NoError(t, Trigger(ctx, trigger, id, nil))
		check(trigger, id, rv)
	}

	// Normal operation; a single pubsub round trip.
	triggerAndCheck(TRIGGER_NIGHTLY, uuid.New().String(), true)

	// Initial handling fails, the message will be delivered again.
	id := uuid.New().String()
	triggerAndCheck(TRIGGER_NIGHTLY, id, false)
	check(TRIGGER_NIGHTLY, id, true)
}
