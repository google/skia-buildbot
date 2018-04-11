package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/push/go/types"
	compute "google.golang.org/api/compute/v1"
)

// ByMetadata triggers a pulld instance to look for new packages by writing a
// value to the 'pushrev' instance metadata, which pulld waits for changes on
// that metadata instance value by doing a hanging GET.
//
// 'revValue' is a unique value to write to the 'pushrev' key, which should
// be the full identifier of the package that was updated.
func ByMetadata(comp *compute.Service, project, revValue, serverName, zone string) error {
	// Add a little randomness to the revValue so it's always unique
	// and this forces pulld to check for new packages.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	revValue = fmt.Sprintf("%s %d", revValue, r.Int63())

	inst, err := comp.Instances.Get(project, zone, serverName).Do()
	if err != nil {
		return fmt.Errorf("Could not find metadata fingerprint address: %s", err)
	}
	found := false
	for _, it := range inst.Metadata.Items {
		if it.Key == "pushrev" {
			it.Value = &revValue
			found = true
		}
	}
	if !found {
		inst.Metadata.Items = append(inst.Metadata.Items,
			&compute.MetadataItems{
				Key:   "pushrev",
				Value: &revValue,
			})
	}
	op, err := comp.Instances.SetMetadata(project, zone, serverName, inst.Metadata).Do()
	if err != nil || op.HTTPStatusCode != 200 {
		return fmt.Errorf("Failed to set pushrev for server instance %q: %s", serverName, err)
	}
	return nil
}

// ByPubSub triggers a pulld instance to look for new packages by sending a
// types.Command via pub/sub.
func ByPubSub(ctx context.Context, ps *pubsub.Client, hostName string) error {
	sklog.Infof("Triggering by Pub/Sub.")
	topic := ps.Topic("push_command")
	cmd := types.Command{
		Action:   types.Pull,
		Hostname: hostName,
	}
	b, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("Failed to encode command: %s", err)
	}
	pr := topic.Publish(ctx, &pubsub.Message{
		Data: b,
	})
	<-pr.Ready()
	return nil
}
