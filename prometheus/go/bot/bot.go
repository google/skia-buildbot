package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
)

const (
	BOT_WEBHOOK_METADATA_KEY = "bot_webhooks"
	BOT_NAME                 = "Alertbot"
)

var (
	client *http.Client
)

type Sender struct {
	DisplayName string `json:"displayName"`
}

type Message struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Sender Sender `json:"sender"`
}

func Init() {
	client = httputils.NewTimeoutClient()
}

func Send(body string, room string) error {
	// First look up the chat room webhook address as stored in project level metadata.
	// The bot_webhooks value is a multiline string, where each line is of the form:
	//
	//    botname webhook_url
	//
	botWebhooks := metadata.ProjectGetWithDefault(BOT_WEBHOOK_METADATA_KEY, "")
	if botWebhooks == "" {
		return fmt.Errorf("Failed to find project metadata for %s", BOT_WEBHOOK_METADATA_KEY)
	}
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

	if body == "" {
		body = "*no message*"
	}

	// We've found the room, so compose the message.
	msg := Message{
		Text: body,
		Sender: Sender{
			DisplayName: BOT_NAME,
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("Failed to encode message: %s", err)
	}
	buf := bytes.NewBuffer(b)

	// Now send the message to the webhook.
	resp, err := client.Post(url, "application/json", buf)
	if err != nil {
		return fmt.Errorf("Failed to send encoded message: %s", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Wrong status code sending message: %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}
