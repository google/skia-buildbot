// chatbot is a package for creating chatbots that interact via webhooks.
package chatbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/sklog"
)

const (
	BOT_WEBHOOK_METADATA_KEY = "bot_webhooks"
)

var (
	client  *http.Client
	botName string
)

type Sender struct {
	DisplayName string `json:"displayName"`
}

type Message struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Sender Sender `json:"sender"`
}

func Init(name string) {
	client = httputils.NewTimeoutClient()
	botName = name
}

// Send the 'body' as a message to the given chat 'room' name.
func Send(body string, room string) error {
	// First look up the chat room webhook address as stored in project level
	// metadata.  The list of supported webhooks is stored at
	// BOT_WEBHOOK_METADATA_KEY in the metadata, and is a multiline string of the
	// form:
	//
	//    botname_1 webhook_url_1 \n
	//    botname_2 webhook_url_2 \n
	//    botname_3 webhook_url_3 \n
	//
	// Note that we load the metadata every time through this func, since loading
	// metadata is very fast, and we expect the message rate to be very low. This
	// ensures we always have a fresh set of bots.
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
	sklog.Infof("Sending to: %q", url)

	body = strings.TrimSpace(body)
	if body == "" {
		body = "*no message*"
	}

	// We've found the room, so compose the message.
	msg := Message{
		Text: body,
		Sender: Sender{
			DisplayName: botName,
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
