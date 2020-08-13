package eventsource

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"

	"go.skia.org/infra/go/httputils"
)

// EventSource is a struct used for HTML5 server-sent events.
type EventSource struct {
	mtx     sync.Mutex
	clients map[chan<- []byte]bool
}

// New returns an EventSource instance.
func New() *EventSource {
	return &EventSource{
		clients: map[chan<- []byte]bool{},
	}
}

// Send the event to all active listeners.
func (m *EventSource) Send(id, event string, data []byte) {
	var buf bytes.Buffer
	if id != "" {
		buf.WriteString(fmt.Sprintf("id: %s\n", id))
	}
	if event != "" {
		buf.WriteString(fmt.Sprintf("event: %s\n", event))
	}
	if len(data) != 0 {
		buf.WriteString(fmt.Sprintf("data: %s\n", string(data)))
	}
	buf.WriteString("\n")
	msg := buf.Bytes()
	m.mtx.Lock()
	defer m.mtx.Unlock()
	for client, _ := range m.clients {
		client <- msg
	}
}

// Handler is the http.HandlerFunc used to listen for events.
func (m *EventSource) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			httputils.ReportError(w, r, fmt.Errorf("Client does not support streaming."), "Client does not support streaming.")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Cache-Control", "no-cache")
		ch := make(chan []byte)
		m.addClient(ch)
		defer func() {
			m.removeClient(ch)
		}()
		for {
			msg := <-ch
			if _, err := w.Write(msg); err != nil {
				http.Error(w, "Failed to write response.", http.StatusInternalServerError)
				return
			}
			flusher.Flush()
		}
	}
}

// Add the given channel to the client map.
func (m *EventSource) addClient(ch chan<- []byte) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.clients[ch] = true
}

// Remove the given channel from the client map.
func (m *EventSource) removeClient(ch chan<- []byte) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	close(ch)
	delete(m.clients, ch)
}
