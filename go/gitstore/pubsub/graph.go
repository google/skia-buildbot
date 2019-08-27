package pubsub

import (
	"context"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/git/repograph"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
)

// AutoUpdateMap is a wrapper around repograph.Map which provides a convenience
// method for auto-updating the Graphs in the Map.
type AutoUpdateMap struct {
	btConf  *bt_gitstore.BTConfig
	Map     repograph.Map
	repoIDs map[string]int64
}

// NewBTGitStoreMap is a wrapper around bt_gitstore.NewBTGitStoreMap which
// provides a convenience method for auto-updating the Graphs in the Map.
func NewAutoUpdateMap(ctx context.Context, repoUrls []string, btConf *bt_gitstore.BTConfig) (*AutoUpdateMap, error) {
	rv := &AutoUpdateMap{
		btConf:  btConf,
		Map:     make(map[string]*repograph.Graph, len(repoUrls)),
		repoIDs: make(map[string]int64, len(repoUrls)),
	}
	for _, repoUrl := range repoUrls {
		gs, err := bt_gitstore.New(ctx, btConf, repoUrl)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create GitStore for %s", repoUrl)
		}
		graph, err := gitstore.GetRepoGraph(ctx, gs)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to create Graph from GitStore for %s", repoUrl)
		}
		rv.Map[repoUrl] = graph
		rv.repoIDs[repoUrl] = gs.RepoID
	}
	return rv, nil
}

// Start initializes auto-updating of the AutoUpdateMap.
func (m *AutoUpdateMap) Start(ctx context.Context, subscriberID string, ts oauth2.TokenSource, fallbackInterval time.Duration, callback AutoUpdateMapCallback) error {
	for repoUrl, graph := range m.Map {
		// https://golang.org/doc/faq#closures_and_goroutines
		repoUrl := repoUrl
		graph := graph
		err := updateUsingPubSub(ctx, m.btConf, subscriberID, m.repoIDs[repoUrl], graph, ts, fallbackInterval, func(ctx context.Context, g *repograph.Graph, ack, nack func()) error {
			return callback(ctx, repoUrl, g, ack, nack)
		})
		if err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// AutoUpdateCallback is a callback function used in UpdateUsingPubSub which is
// called after the Graph is updated but before the changes are committed. If
// the callback returns an error, the changes are not committed. In addition to
// the Graph itself, the callback accepts two functions as parameters, ack and
// nack, which in turn call the Ack() or Nack() functions on the pubsub
// message(s) which triggered the update. The callback function should call one
// of them to ensure that the message(s) get redelivered or not, as desired.
type AutoUpdateCallback func(ctx context.Context, g *repograph.Graph, ack, nack func()) error

// AutoUpdateMapCallback is like AutoUpdateCallback, except that it's handed to
// NewAutoUpdateMap.Start() and also includes the repo URL.
type AutoUpdateMapCallback func(ctx context.Context, repoUrl string, g *repograph.Graph, ack, nack func()) error

// updateUsingPubSub updates the passed-in Graph whenever a pubsub message is
// received for the given repo and at the given fallback interval. It calls the
// given callback function after the Graph updates but before the changes are
// committed.
func updateUsingPubSub(ctx context.Context, btConf *bt_gitstore.BTConfig, subscriberID string, repoID int64, graph *repograph.Graph, ts oauth2.TokenSource, fallbackInterval time.Duration, callback AutoUpdateCallback) error {
	var ticker *time.Ticker
	var tickCh <-chan time.Time
	if fallbackInterval > 0 {
		ticker = time.NewTicker(fallbackInterval)
		go func() {
			<-ctx.Done()
			ticker.Stop()
		}()
		tickCh = ticker.C
	}
	_, err := updateUsingPubSubHelper(ctx, btConf, subscriberID, repoID, graph, ts, tickCh, callback)
	if err != nil && ticker != nil {
		ticker.Stop()
	}
	return err
}

// ackFn returns a function which Acks the message.
func ackFn(msg *pubsub.Message) func() {
	if msg == nil {
		return func() {}
	}
	return msg.Ack
}

// nackFn returns a function which Nacks all of the messages in the batch.
func nackFn(msg *pubsub.Message) func() {
	if msg == nil {
		return func() {}
	}
	return msg.Nack
}

// updateUsingPubSubHelper is a helper function used by UpdateUsingPubSub, for
// testing. In addition to any error, returns a func which waits for all
// spawned goroutines to exit.
func updateUsingPubSubHelper(ctx context.Context, btConf *bt_gitstore.BTConfig, subscriberID string, repoID int64, graph *repograph.Graph, ts oauth2.TokenSource, tickCh <-chan time.Time, callback AutoUpdateCallback) (func(), error) {
	var wg sync.WaitGroup
	var mtx sync.Mutex
	doUpdate := func(ctx context.Context, msg *pubsub.Message) {
		mtx.Lock()
		defer mtx.Unlock()
		err := graph.UpdateWithCallback(ctx, func(g *repograph.Graph) error {
			return callback(ctx, g, ackFn(msg), nackFn(msg))
		})
		if err != nil {
			sklog.Errorf("Failed to update repo: %s", err)
		}
	}

	// Create the PubSub subscription.
	err := NewSubscriber(ctx, btConf, subscriberID, repoID, ts, func(msg *pubsub.Message, branches map[string]string) {
		doUpdate(ctx, msg)
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create auto-updating repograph.Graph")
	}

	// Spin up a goroutine to update the Graph at the specified fallback
	// interval.
	if tickCh != nil {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
			}()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tickCh:
					doUpdate(ctx, nil)
				}
			}
		}()
	}
	return wg.Wait, nil
}
