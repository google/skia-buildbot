package geventbus

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bitly/go-nsq"
	"github.com/hashicorp/golang-lru"
	"github.com/satori/go.uuid"
	"go.skia.org/infra/go/util"
)

type GlobalEventBus interface {
	// Publish to a topic globally.
	Publish(topic string, data []byte) error

	// Subscribe to a topic. The callback function will be called on
	// its own go-routine.
	SubscribeAsync(topic string, callback CallbackFn) error

	// DispatchSentMessages sets a flag whether to dispatch messages
	// send through this instance to subscribers. This is necessary to
	// prevent events from being sent twice if they are also dispatched via
	// a local event bus.
	DispatchSentMessages(newVal bool)

	// Close gracefully shuts down all open connections.
	Close() error
}

/*
	NSQEventBus implements the GlobalEventBus interface.
	It uses NSQ for message transport (see http://nsq.io/).
	NSQ allows to publish to an arbitary number of topcis. Each topic can have
	an arbitrary number of channels.

	In our use case we publish to a topic (identified by a string) and each
	client creates a unique channel, which ensures the topic messages are
	distributed to all clients (as opposed to being load balanced accross a single
  channel).

	By appending '#ephemeral' to the channel id we ensure that a channel
	will never be buffered on disk. We could relax this requirement in the
	future if we have constant channel ids that are guaranteed to
	connect to the channel continously and retrieve buffered messages.

*/
type NSQEventBus struct {
	// Unique id identifying this client.
	clientID string

	// Address of the nsqd that relays messages.
	address string

	// NSQ configuration shared between consumers and producers.
	config *nsq.Config

	// NSQ producer used to publish events.
	producer *nsq.Producer

	// Unique prefix prepended to each message to recognize whether a message
	// was sent by this instance.
	producerPrefix string

	// MessageIdCounter is atomically incremented counter that uniquely identifies
	// a message by this producer.
	messageIdCounter *int64
	dedupCache       *lru.Cache

	// Tracks whether to dispatch events to subscribers that were sent by
	// this instance.
	dispatchSent bool

	// consumerCallbacks map [topic] to an nsq consumer and the topic callbacks.
	consumerCallbacks map[string]*consumerCallbackT

	// mutex protects consumerCallbacks.
	mutex sync.Mutex
}

// Maximum size of the LRU cache for message deduplication.
const MAX_CACHE_SIZE = 20000

// Separator used to distinguish parts of the prefix: sender_id : message_id : payload.
const PREFIX_SEPARATOR = ":"

// consumberCallbackT aggregates the nsq consumer and the callback functions for
// a single topic.
type consumerCallbackT struct {
	consumer  *nsq.Consumer
	callbacks []CallbackFn
}

// CallbackFn defines the signature of all callback functions to
// handle subscription.
type CallbackFn func(data []byte)

// NewNSQEventBus returns a new instance of NSQEventBus.
// 'address' is the address (hostname:port) of the nsqd instance that relays the
// messages.
func NewNSQEventBus(address string) (GlobalEventBus, error) {
	// Create a client id based on timestamp, mac address and a random string.
	clientID := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	producerPrefix := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()

	// Keeps track of individual messages.
	messageIdCounter := new(int64)
	*messageIdCounter = time.Now().Unix()
	dedupCache, err := lru.New(MAX_CACHE_SIZE)
	if err != nil {
		return nil, err
	}

	config := nsq.NewConfig()
	producer, err := nsq.NewProducer(address, config)
	if err != nil {
		return nil, err
	}

	if err := producer.Ping(); err != nil {
		return nil, err
	}

	ret := &NSQEventBus{
		clientID:          clientID,
		address:           address,
		config:            config,
		producer:          producer,
		producerPrefix:    producerPrefix,
		messageIdCounter:  messageIdCounter,
		dedupCache:        dedupCache,
		dispatchSent:      true,
		consumerCallbacks: map[string]*consumerCallbackT{},
	}

	return ret, nil
}

// See GlobalEventBus interface.
func (g *NSQEventBus) Publish(topic string, data []byte) error {
	messageID := strconv.FormatInt(atomic.AddInt64(g.messageIdCounter, 1), 10)
	msg := strings.Join([]string{g.producerPrefix, messageID, string(data)}, PREFIX_SEPARATOR)
	return g.producer.Publish(topic, []byte(msg))
}

// See GlobalEventBus interface.
func (g *NSQEventBus) SubscribeAsync(topic string, callback CallbackFn) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	ccb, ok := g.consumerCallbacks[topic]
	if !ok {
		consumer, err := nsq.NewConsumer(topic, g.clientID+"#ephemeral", g.config)
		if err != nil {
			return err
		}
		consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
			// Get the sender from the prefix and only dispatch if the dispatchSent flag
			// is set.
			splitMessage := strings.SplitN(string(message.Body), PREFIX_SEPARATOR, 3)
			if !g.dispatchSent && (splitMessage[0] == g.producerPrefix) {
				return nil
			}

			// Check if we have seen this message already.
			found, _ := g.dedupCache.ContainsOrAdd(splitMessage[0]+PREFIX_SEPARATOR+splitMessage[1], true)
			if found {
				return nil
			}

			// Ensure we don't collide with subscriptions. This should be the exception
			// since most of the subscription will be done during app setup.
			g.mutex.Lock()
			defer g.mutex.Unlock()
			for _, cb := range g.consumerCallbacks[topic].callbacks {
				go cb([]byte(splitMessage[2]))
			}
			return nil
		}))
		if err := consumer.ConnectToNSQD(g.address); err != nil {
			return err
		}
		ccb = &consumerCallbackT{
			consumer:  consumer,
			callbacks: []CallbackFn{},
		}
		g.consumerCallbacks[topic] = ccb
	}

	ccb.callbacks = append(ccb.callbacks, callback)
	return nil
}

// See GlobalEventBus interface.
func (g *NSQEventBus) Close() error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for _, ccb := range g.consumerCallbacks {
		ccb.consumer.Stop()
		<-ccb.consumer.StopChan
	}
	g.consumerCallbacks = nil
	g.producer.Stop()
	return nil
}

// See GlobalEventBus interface.
func (n *NSQEventBus) DispatchSentMessages(newVal bool) {
	n.dispatchSent = newVal
}

// JSONCallback is an adapter between a CallbackFn and a typed function
// that deals with deserialized JSON data.
// Example:
//
//  fn := JSONCallback(&MyType{}, func(data interface{}, err error) {
//     ... data.(*MyType)
//  })
//  fn(jsonBytes)
//
// This assumes that jsonBytes is valid JSON to deserialize to an
// instance of MyType.
//
func JSONCallback(instance interface{}, callback func(data interface{}, err error)) CallbackFn {
	codec := util.JSONCodec(instance)
	return func(byteData []byte) {
		callback(codec.Decode(byteData))
	}
}
