package geventbus

import (
	"sync"

	"github.com/bitly/go-nsq"
	"github.com/satori/go.uuid"
)

// GlobalEventBus is a distributed eventbus based on NSQ.
// It allows to subscribe to and publish events across a network.
type GlobalEventBus struct {
	// Unique id identifying this client.
	clientID string

	// Address of the nsqd that relays messages.
	address string

	// NSQ configuration shared between consumers and producers.
	config *nsq.Config

	// NSQ producer used to publish events.
	producer *nsq.Producer

	// consumerCallbacks map [topic] to an nsq consumer and the topic callbacks.
	consumerCallbacks map[string]*consumerCallbackT

	// mutex protects consumerCallbacks.
	mutex sync.Mutex
}

// consumberCallbackT aggregates the nsq consumer and the callback functions for
// a single topic.
type consumerCallbackT struct {
	consumer  *nsq.Consumer
	callbacks []CallbackFn
}

// CallbackFn defines the signature of all callback functions to handle subscription.
type CallbackFn func(data []byte)

// NewGlobalEventBus returns a new instance of GlobalEventBus.
// 'address' is the address (hostname:port) of the nsqd instance that relays the
// messages.
func NewGlobalEventBus(address string) (*GlobalEventBus, error) {
	// Create a client id based on timestamp, mac address and a random string.
	clientID := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()

	config := nsq.NewConfig()
	producer, err := nsq.NewProducer(address, config)
	if err != nil {
		return nil, err
	}

	if err := producer.Ping(); err != nil {
		return nil, err
	}

	ret := &GlobalEventBus{
		clientID:          clientID,
		address:           address,
		config:            config,
		producer:          producer,
		consumerCallbacks: map[string]*consumerCallbackT{},
	}

	return ret, nil
}

// Publish to a topic.
func (g *GlobalEventBus) Publish(topic string, data []byte) error {
	return g.producer.Publish(topic, data)
}

// Subscribe to a topic. The callback function will be called on its own go-routine.
func (g *GlobalEventBus) SubscribeAsync(topic string, callback CallbackFn) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	ccb, ok := g.consumerCallbacks[topic]
	if !ok {
		consumer, err := nsq.NewConsumer(topic, g.clientID, g.config)
		if err != nil {
			return err
		}
		consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
			// Ensure we don't collide with subscriptions. This should be the exception
			// since most of the subscription will be done during app setup.
			g.mutex.Lock()
			defer g.mutex.Unlock()

			for _, cb := range g.consumerCallbacks[topic].callbacks {
				go cb(message.Body)
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

// Close gracefully shuts down all open connections.
func (g *GlobalEventBus) Close() {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	for _, ccb := range g.consumerCallbacks {
		ccb.consumer.Stop()
	}
	g.consumerCallbacks = nil
	g.producer.Stop()
}
