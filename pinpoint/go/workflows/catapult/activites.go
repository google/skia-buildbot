package catapult

import (
	"context"

	"go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/backends"
)

// FetchTaskActivity fetches the task used for the given swarming task.
func FetchTaskActivity(ctx context.Context, taskID string) (*swarming.SwarmingRpcsTaskResult, error) {
	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	task, err := sc.GetTask(ctx, taskID, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not fetch task %s", taskID)
	}

	return task, nil
}
