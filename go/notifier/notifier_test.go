package notifier

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestConfigs(t *testing.T) {
	testutils.SmallTest(t)

	c := Config{}
	assert.EqualError(t, c.Validate(), "Either Filter or MsgTypeWhitelist is required.")

	c = Config{
		Filter: "bogus",
	}
	assert.EqualError(t, c.Validate(), "Unknown filter \"bogus\"")

	c = Config{
		Filter:           "debug",
		MsgTypeWhitelist: []string{"whitelisted-type"},
	}
	assert.EqualError(t, c.Validate(), "Only one of Filter or MsgTypeWhitelist may be provided.")

	c = Config{
		Filter: "debug",
	}
	assert.EqualError(t, c.Validate(), "Exactly one notification config must be supplied, but got 0")

	c = Config{
		Filter: "debug",
		Email:  &EmailNotifierConfig{},
	}
	assert.EqualError(t, c.Validate(), "Emails is required.")

	c = Config{
		Filter: "debug",
		Email: &EmailNotifierConfig{
			Emails: []string{},
		},
	}
	assert.EqualError(t, c.Validate(), "Emails is required.")

	c = Config{
		Filter: "debug",
		Email: &EmailNotifierConfig{
			Emails: []string{"test@example.com"},
		},
	}
	assert.NoError(t, c.Validate())

	c = Config{
		Filter: "debug",
		Chat:   &ChatNotifierConfig{},
	}
	assert.EqualError(t, c.Validate(), "RoomID is required.")

	c = Config{
		Filter: "debug",
		Chat: &ChatNotifierConfig{
			RoomID: "my-room",
		},
	}
	assert.NoError(t, c.Validate())

	c = Config{
		Filter: "debug",
		Email: &EmailNotifierConfig{
			Emails: []string{"test@example.com"},
		},
		Chat: &ChatNotifierConfig{},
	}
	assert.EqualError(t, c.Validate(), "Exactly one notification config must be supplied, but got 2")

	c = Config{
		MsgTypeWhitelist: []string{"filebug"},
		Monorail:         &MonorailNotifierConfig{},
	}
	assert.EqualError(t, c.Validate(), "Owner is required.")

	c = Config{
		MsgTypeWhitelist: []string{"filebug"},
		Monorail: &MonorailNotifierConfig{
			Owner: "me",
		},
	}
	assert.EqualError(t, c.Validate(), "Project is required.")

	c = Config{
		MsgTypeWhitelist: []string{"filebug"},
		Monorail: &MonorailNotifierConfig{
			Owner:   "me",
			Project: "my-project",
		},
	}
	assert.NoError(t, c.Validate())
}
