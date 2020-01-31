package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"

	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/niagara/go/db"
	"go.skia.org/infra/niagara/go/messages"
)

const (
	pubsubTopic = "niagara-machines-skia"

	maxFirestoreWriteAttempts = 5
	maxFirestoreOperationTime = 2 * time.Minute
)

func main() {
	flag.Parse()
	firestore.EnsureNotEmulator()
	fmt.Println("hello machine manager")
	ctx := context.Background()

	// Auth note: the underlying firestore.NewClient looks at the
	// GOOGLE_APPLICATION_CREDENTIALS env variable, so we don't need to supply
	// a token source.
	fsClient, err := firestore.NewClient(ctx, "skia-firestore", "niagara", "testing", nil)
	if err != nil {
		sklog.Fatalf("Unable to configure Firestore: %s", err)
	}
	sklog.Infof("Firestore good %v\n", fsClient)

	psClient, err := pubsub.NewClient(ctx, "skia-public")
	if err != nil {
		sklog.Fatalf("Unable to configure Pubsub: %s", err)
	}
	sub, err := setupTopicSubscription(ctx, psClient, pubsubTopic, "machine-manager-01")
	if err != nil {
		sklog.Fatalf("Unable to setups subscription: %s", err)
	}

	sklog.Infof("PubSub topic/subscription set up %v\n", sub)
	m := Manager{
		client: fsClient,
	}

	sklog.Fatalf("error while receiving pubsub notifications: %s", sub.Receive(ctx, m.processMachineMessage))
}

type Manager struct {
	client *firestore.Client
}

func (m *Manager) processMachineMessage(ctx context.Context, msg *pubsub.Message) {
	msg.Ack()

	var state messages.MachineState
	if err := json.Unmarshal(msg.Data, &state); err != nil {
		sklog.Errorf("Invalid message data %s: %s", err, string(msg.Data))
		return
	}
	mID := msg.Attributes[messages.MachineID]
	if mID == "" {
		sklog.Errorf("Invalid message - no id: %s", msg)
		return
	}
	event := messages.MachineEvent(msg.Attributes[messages.Event])
	if event == messages.Booted {
		// TODO(kjlubick) Check that either we haven't seen this machine before, or the previous
		//  state was rebooting or sitting idle
		// TODO(kjlubick) check the health of this machine.
		if err := m.updateMachine(ctx, mID, messages.Ready, messages.Booted, state); err != nil {
			sklog.Errorf("Could not update machine entry %s", err)
		}
	}

	sklog.Infof("Got message %s with data %s", msg.ID, string(msg.Data))
}

func (m *Manager) updateMachine(ctx context.Context, machineID string, newStatus messages.MachineStatus, currEvent messages.MachineEvent, state messages.MachineState) error {
	fme := db.FirestoreMachineEntry{
		State:       newStatus,
		LastEvent:   currEvent,
		Updated:     time.Now(),
		Dimensions:  state.Dimensions,
		CurrentTask: state.CurrentTask,
	}
	// store to firestore
	doc := m.client.Collection("machines").Doc(machineID)
	if _, err := m.client.Set(ctx, doc, &fme, maxFirestoreWriteAttempts, maxFirestoreOperationTime); err != nil {
		return skerr.Wrapf(err, "updating machine entry for %s", machineID)
	}
	return nil
}

func setupTopicSubscription(ctx context.Context, psClient *pubsub.Client, topicName, subscriberName string) (*pubsub.Subscription, error) {
	// Create the topic if it doesn't exist yet.
	topic := psClient.Topic(topicName)
	if exists, err := topic.Exists(ctx); err != nil {
		return nil, skerr.Wrapf(err, "checking whether topic %s exists", topicName)
	} else if !exists {
		if topic, err = psClient.CreateTopic(ctx, topicName); err != nil {
			return nil, skerr.Wrapf(err, "creating pubsub topic '%s'", topicName)
		}
	}

	// Create the subscription if it doesn't exist.
	subName := fmt.Sprintf("%s+%s", subscriberName, topicName)
	sub := psClient.Subscription(subName)
	if exists, err := sub.Exists(ctx); err != nil {
		return nil, skerr.Wrapf(err, "checking existence of pubsub subscription '%s'", subName)
	} else if !exists {
		sub, err = psClient.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic:             topic,
			AckDeadline:       10 * time.Second,
			RetentionDuration: time.Hour,
			ExpirationPolicy:  time.Duration(0),
		})
		if err != nil {
			return nil, skerr.Wrapf(err, "creating pubsub subscription '%s'", subName)
		}
	}
	sub.ReceiveSettings.MaxOutstandingMessages = 100
	sub.ReceiveSettings.NumGoroutines = 10
	return sub, nil
}
