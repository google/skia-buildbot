package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	project      = flag.String("project_id", "", "GCE Project ID")
	topicName    = flag.String("topic", "", "Pubsub topic name")
	file         = flag.String("file", "", "File from which to read/write pubsub messages.")
	record       = flag.Bool("record", false, "If true, record pubsub messages and write them to the given file.")
	playback     = flag.Bool("playback", false, "If true, read messages from the given file and send them via pubsub.")
	waitForInput = flag.Bool("wait_for_input", false, "If true, wait for user input before playing back each message in playback mode.")
)

func main() {
	// Initial setup.
	common.Init()
	if *project == "" {
		sklog.Fatal("--project_id is required.")
	}
	if *topicName == "" {
		sklog.Fatal("--topic is required.")
	}
	if *file == "" {
		sklog.Fatal("--file is required.")
	}
	if *record == *playback {
		sklog.Fatal("Exactly one of --playback or --record is required.")
	}

	// Setup pubsub.
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, *project)
	if err != nil {
		sklog.Fatal(err)
	}
	topic := client.Topic(*topicName)
	if exists, err := topic.Exists(ctx); err != nil {
		sklog.Fatal(err)
	} else if !exists {
		topic, err = client.CreateTopic(ctx, *topicName)
		if err != nil {
			sklog.Fatal(err)
		}
	}

	// Perform the desired action.
	if *record {
		doRecord(ctx, client, *file)
	} else {
		doPlayback(ctx, client, *file)
	}
}

// doRecord records pubsub messages until the process is interrupted.
func doRecord(ctx context.Context, client *pubsub.Client, file string) {
	// Create a subscription.
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	subName := fmt.Sprintf("pubsub_recorder_%s", hostname)
	sub := client.Subscription(subName)
	if exists, err := sub.Exists(ctx); err != nil {
		sklog.Fatal(err)
	} else if exists {
		if err := sub.Delete(ctx); err != nil {
			sklog.Fatal(err)
		}
	}
	sub, err = client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
		Topic:       client.Topic(*topicName),
		AckDeadline: 10 * time.Second,
	})
	if err != nil {
		sklog.Fatal(err)
	}
	defer func() {
		if err := sub.Delete(ctx); err != nil {
			sklog.Fatalf("Failed to delete subscription: %s", err)
		}
	}()

	// Receive messages one at a time and write them to the file. Use a
	// mutex to serialize writes to the backing file.
	sklog.Infof("Waiting to receive pubsub messages.")
	msgs := []*pubsub.Message{}
	var mtx sync.Mutex
	if err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		mtx.Lock()
		defer mtx.Unlock()
		sklog.Infof("Processing pubsub message %s", msg.ID)
		msgs = append(msgs, msg)
		if err := util.WithWriteFile(file, func(w io.Writer) error {
			return json.NewEncoder(w).Encode(msgs)
		}); err != nil {
			sklog.Fatal(err)
		}
		msg.Ack()
	}); err != nil {
		sklog.Fatal(err)
	}
}

// doPlayback plays back all pubsub messages in the given file.
func doPlayback(ctx context.Context, client *pubsub.Client, file string) {
	topic := client.Topic(*topicName)
	defer topic.Stop()
	b, err := ioutil.ReadFile(file)
	if err != nil {
		sklog.Fatal(err)
	}
	var msgs []*pubsub.Message
	if err := json.Unmarshal(b, &msgs); err != nil {
		sklog.Fatal(err)
	}
	for _, msg := range msgs {
		if *waitForInput {
			sklog.Infof("Press enter key to send next message...")
			if _, err := bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
				sklog.Fatal(err)
			}
		}
		sklog.Infof("Publishing message %s", msg.ID)
		if _, err := topic.Publish(ctx, msg).Get(ctx); err != nil {
			sklog.Fatal(err)
		}
	}
}
