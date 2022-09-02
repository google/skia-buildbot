package notifier

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

func TestConfigs(t *testing.T) {

	c := Config{}
	require.EqualError(t, c.Validate(), "Either Filter or IncludeMsgTypes is required.")

	c = Config{
		Filter: "bogus",
	}
	require.EqualError(t, c.Validate(), "Unknown filter \"bogus\"")

	c = Config{
		Filter:          "debug",
		IncludeMsgTypes: []string{"included-type"},
	}
	require.EqualError(t, c.Validate(), "Only one of Filter or IncludeMsgTypes may be provided.")

	c = Config{
		Filter: "debug",
	}
	require.EqualError(t, c.Validate(), "Exactly one notification config must be supplied, but got 0")

	c = Config{
		Filter: "debug",
		Email:  &EmailNotifierConfig{},
	}
	require.EqualError(t, c.Validate(), "Emails is required.")

	c = Config{
		Filter: "debug",
		Email: &EmailNotifierConfig{
			Emails: []string{},
		},
	}
	require.EqualError(t, c.Validate(), "Emails is required.")

	c = Config{
		Filter: "debug",
		Email: &EmailNotifierConfig{
			Emails: []string{"test@example.com"},
		},
	}
	require.NoError(t, c.Validate())

	c = Config{
		Filter: "debug",
		Chat:   &ChatNotifierConfig{},
	}
	require.EqualError(t, c.Validate(), "RoomID is required.")

	c = Config{
		Filter: "debug",
		Chat: &ChatNotifierConfig{
			RoomID: "my-room",
		},
	}
	require.NoError(t, c.Validate())

	c = Config{
		Filter: "debug",
		Email: &EmailNotifierConfig{
			Emails: []string{"test@example.com"},
		},
		Chat: &ChatNotifierConfig{},
	}
	require.EqualError(t, c.Validate(), "Exactly one notification config must be supplied, but got 2")

	c = Config{
		IncludeMsgTypes: []string{"filebug"},
		Monorail:        &MonorailNotifierConfig{},
	}
	require.EqualError(t, c.Validate(), "Project is required.")

	c = Config{
		IncludeMsgTypes: []string{"filebug"},
		Monorail:        &MonorailNotifierConfig{},
	}
	require.EqualError(t, c.Validate(), "Project is required.")

	c = Config{
		IncludeMsgTypes: []string{"filebug"},
		Monorail: &MonorailNotifierConfig{
			Project: "my-project",
		},
	}
	require.NoError(t, c.Validate())
}

func TestConfigCopy(t *testing.T) {

	c := &Config{
		Filter:          "info",
		IncludeMsgTypes: []string{"a", "b"},
		Subject:         "blah blah",
		Chat: &ChatNotifierConfig{
			RoomID: "my-room",
		},
		Email: &EmailNotifierConfig{
			Emails: []string{"me@google.com", "you@google.com"},
		},
		Monorail: &MonorailNotifierConfig{
			Project:    "my-project",
			Owner:      "me",
			CC:         []string{"you", "me"},
			Components: []string{"my-component"},
			Labels:     []string{"a", "b"},
		},
		PubSub: &PubSubNotifierConfig{
			Topic: "my-topic",
		},
	}
	cpy := c.Copy()
	assertdeep.Copy(t, c, cpy)

	// Note: AssertCopy does not dig into the member structs to see if those
	// have also been properly initialized for testing. Call AssertCopy on
	// each member struct to verify that we properly initialized them.
	assertdeep.Copy(t, c.Chat, cpy.Chat)
	assertdeep.Copy(t, c.Email, cpy.Email)
	assertdeep.Copy(t, c.Monorail, cpy.Monorail)
	assertdeep.Copy(t, c.PubSub, cpy.PubSub)
}
