package webevent

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

type EventType string

const (
	Update = "update"
	Error  = "error"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type EventEmitterFn func(r *http.Request, conn *websocket.Conn)

type EventEnvelope struct {
	Error string      `json:"error"`
	Data  interface{} `json:"data"`
}

func Handle(emitterFn EventEmitterFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			httputils.ReportError(w, r, err, "Unable to upgrade web socket")
			return
		}
		emitterFn(r, conn)
	}
}

func ReportError(conn *websocket.Conn, publicErrorMsg string, err error) {
}

type EventDispatcher struct {
	eventBus      eventbus.EventBus
	feeds         map[string]*eventFeed
	subscriptions map[string]*Subscription
	mutex         sync.Mutex
}

func NewEventDispatcher(eventBus eventbus.EventBus) *EventDispatcher {
	return &EventDispatcher{
		eventBus: eventBus,
		feeds:    map[string]*eventFeed{},
	}
}

func (e *EventDispatcher) Subscribe(eventTypes ...string) (*Subscription, error) {
	// At most keep the last 10 events. See eventFeed.handleEvent method.
	eventCh := make(chan *Event, 10)
	uuidBytes, err := uuid.NewRandom()
	if err != nil {
		return nil, sklog.FmtErrorf("Unable to generate UUID: %s", err)
	}
	id := uuidBytes.String()

	ret := &Subscription{
		Channel:    eventCh,
		id:         id,
		eventTypes: eventTypes,
	}

	e.addSubscription(ret)
	return ret, nil
}

func (e *EventDispatcher) addSubscription(subscription *Subscription) {
	for _, evtType := range subscription.eventTypes {
		feed := e.getFeed(evtType)
		feed.add(subscription)
	}
}

func (e *EventDispatcher) removeSubscription(subscription *Subscription) {
	for _, evtType := range subscription.eventTypes {
		e.mutex.Lock()
		feed, ok := e.feeds[evtType]
		e.mutex.Unlock()
		if !ok {
			continue
		}
		feed.remove(subscription.id)
	}
}

func (e *EventDispatcher) getFeed(eventType string) *eventFeed {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if feed, ok := e.feeds[eventType]; ok {
		return feed
	}
	feed := &eventFeed{
		eventType:     eventType,
		subscriptions: map[string]chan *Event{},
	}
	e.eventBus.SubscribeAsync(eventType, feed.handleEvent)
	e.feeds[eventType] = feed
	return feed
}

type eventFeed struct {
	eventType     string
	subscriptions map[string]chan *Event
	mutex         sync.Mutex
}

func (ef *eventFeed) add(sub *Subscription) {
	ef.mutex.Lock()
	defer ef.mutex.Unlock()
	ef.subscriptions[sub.id] = sub.Channel
}

func (ef *eventFeed) remove(subscriptionID string) {
	ef.mutex.Lock()
	defer ef.mutex.Unlock()
	delete(ef.subscriptions, subscriptionID)
}

func (ef *eventFeed) handleEvent(evtData interface{}) {
	// Wrap the event data with it's type.
	evtWrapper := &Event{
		Type: ef.eventType,
		Data: evtData,
	}

	ef.mutex.Lock()
	defer ef.mutex.Unlock()
	for _, ch := range ef.subscriptions {
		for {
			select {
			case ch <- evtWrapper:
				break
			default:
				// If we cannot write to the buffered channel, drop the first message and try again.
				<-ch
			}
		}
	}
}

type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type Subscription struct {
	Channel    chan *Event
	id         string
	eventTypes []string
	dispatcher *EventDispatcher
}

func (s *Subscription) Cancel() {
	s.dispatcher.removeSubscription(s)
	drainEventChannel(s.Channel)
	close(s.Channel)
}

func drainEventChannel(ch chan *Event) {
	for {
		select {
		case <-ch:
		default:
			break
		}
	}
}

func SendEvent(conn *websocket.Conn, eventType EventType, payload interface{}) error {
	evt := &Event{
		Type: string(eventType),
		Data: payload,
	}
	w, err := conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return sklog.FmtErrorf("Error getting writer: %s", err)
	}
	if err := json.NewEncoder(w).Encode(evt); err != nil {
		util.Close(w)
		return sklog.FmtErrorf("Error encoding JSON: %s", err)
	}
	return w.Close()
}
