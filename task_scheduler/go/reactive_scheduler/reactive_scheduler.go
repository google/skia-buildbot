package reactive_scheduler

import (
	"context"

	"cloud.google.com/go/firestore"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/task_scheduler/go/blacklist"
	fs "go.skia.org/infra/task_scheduler/go/db/firestore"
)

func Start(ctx context.Context, db *fs.FirestoreDB) error {
	return nil
}

func JobsChanged(snap *firestore.QuerySnapshot, db *fs.FirestoreDB) error {
	return nil
}

func TaskCandidatesChanged(snap *firestore.QuerySnapshot, db *fs.FirestoreDB) error {
	return nil
}

func MaybeTriggerTask(bots []*swarming_api.SwarmingRpcsBotInfo, candidates []*task_candidate.TaskCandidate, db *fs.FirestoreDB) error {
	return nil
}

func BotStatusChanged(bot *swarming_api.SwarmingRpcsBotInfo, db *fs.FirestoreDB) error {
	return nil
}

func TaskFinished(task *swarming_api.SwarmingRpcsTaskResult, db *fs.FirestoreDB) error {
	return nil
}

func BlacklistChanged(bl *blacklist.Blacklist, db *fs.FirestoreDB) error {
	return nil
}
