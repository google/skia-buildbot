// chatbot is a package for creating chatbots that interact via webhooks.
package chatbot

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
func Send(body, room, thread string) error {
	return SendUsingConfig(body, room, thread, func() string {
		return metadata.ProjectGetWithDefault(BOT_WEBHOOK_METADATA_KEY, "")
	})
}

// ConfigReader is a func that returns the config for SendUsingConfig.
type ConfigReader func() string

// SendUsingConfig is just like Send(), but the config retrieved is
// provided by the configReader.
func SendUsingConfig(body, room, thread string, configReader ConfigReader) error {
	// First look up the chat room webhook address as stored in the config.  The
	// list of supported webhooks is a multiline string of the form:
	//
	//    botname_1 webhook_url_1 \n
	//    botname_2 webhook_url_2 \n
	//    botname_3 webhook_url_3 \n
	//
	// Note that we load the config every time through this func, since loading
	// config is very fast, and we expect the message rate to be very low. This
	// ensures we always have a fresh set of bots.
	if configReader == nil {
		return errors.New("No configReader provided; can't send messages.")
	}
	botWebhooks := configReader()
	if botWebhooks == "" {
		return fmt.Errorf("Got empty config.")
	}
	lines := strings.Split(botWebhooks, "\n")
	u := ""
	for _, line := range lines {
		parts := strings.Split(line, " ")
		if len(parts) == 2 && parts[0] == room {
			u = parts[1]
			break
		}
	}
	if u == "" {
		return fmt.Errorf("Unknown room name: %q", room)
	}
	if thread != "" {
		parsedUrl, err := url.Parse(u)
		if err != nil {
			return err
		}
		q := parsedUrl.Query()
		q.Set("thread_key", base64.StdEncoding.EncodeToString([]byte(thread)))
		parsedUrl.RawQuery = q.Encode()
		u = parsedUrl.String()
	}
	sklog.Infof("Sending to: %q", u)

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
	resp, err := client.Post(u, "application/json", buf)
	if err != nil {
		return fmt.Errorf("Failed to send encoded message: %s", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Wrong status code sending message: %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}
