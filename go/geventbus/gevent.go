package geventbus

// CallbackFn defines the signature of all callback functions to
// handle subscription.
type CallbackFn func(data []byte)

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

func New() (GlobalEventBus, error) {
	return nil, nil
}
