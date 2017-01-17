package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/metadata"
)

var (
	client *http.Client
)

type Sender struct {
	Name string `json:"name"`
}

type Message struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Sender Sender `json:"sender"`
}

func Send(body string, room string) error {
	// Parse message
	// Look up the webhook address in metadata.
	// Format message.
	// Send to webhook.

	// The bot_webhooks are of the form:
	//
	// botname webhook_url.
	botWebhooks := metadata.ProjectGetWithDefault("bot_webhooks", "")
	lines := strings.Split(botWebhooks, "\n")
	url := ""
	for _, line := range lines {
		parts := strings.Split(line, " ")
		if len(parts) == 2 && parts[0] == room {
			url = parts[1]
			break
		}
	}
	if url == "" {
		return fmt.Errorf("Unknown room name: %q", room)
	}

	msg := Message{
		Text: body,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Failed to encode message: %s", err)
	}
	buf := bytes.NewBuffer(b)
	resp, err := client.Post(url, "application/json", buf)
	if err != nil {
		return fmt.Errorf("Failed to send encoded message: %s", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Wrong status code sending message: %s", err)
	}
	return nil
}
