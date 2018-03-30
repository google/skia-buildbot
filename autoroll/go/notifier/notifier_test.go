package notifier

import (
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/notifier"
	"go.skia.org/infra/go/testutils"
)

type msg struct {
	subject string
	m       *notifier.Message
}

type testNotifier struct {
	msgs []*msg
}

func (n *testNotifier) Send(subject string, m *notifier.Message) error {
	n.msgs = append(n.msgs, &msg{
		subject: subject,
		m:       m,
	})
	return nil
}

func TestNotifier(t *testing.T) {
	testutils.SmallTest(t)

	n := New("childRepo", "parentRepo", nil)
	t1 := &testNotifier{}
	n.Add(t1, notifier.FILTER_DEBUG, "")

	assert.NoError(t, n.SendIssueUpdate("123", "https://codereview/123", "uploaded a CL!"))
	assert.Equal(t, 1, len(t1.msgs))
	assert.Equal(t, "The childRepo into parentRepo AutoRoller has uploaded issue 123", t1.msgs[0].subject)
	assert.Equal(t, "uploaded a CL!", t1.msgs[0].m.Body)
	assert.Equal(t, notifier.SEVERITY_INFO, t1.msgs[0].m.Severity)

	assert.NoError(t, n.SendModeChange("test@skia.org", "STOPPED", "<b>Staaahhp!</b>"))
	assert.Equal(t, 2, len(t1.msgs))
	assert.Equal(t, "The childRepo into parentRepo AutoRoller mode was changed", t1.msgs[1].subject)
	assert.Equal(t, "test@skia.org changed the mode to \"STOPPED\" with message: &lt;b&gt;Staaahhp!&lt;/b&gt;", t1.msgs[1].m.Body)
	assert.Equal(t, notifier.SEVERITY_WARNING, t1.msgs[1].m.Severity)

	now := time.Now().Round(time.Millisecond)
	assert.NoError(t, n.SendSafetyThrottled(now))
	assert.Equal(t, 3, len(t1.msgs))
	assert.Equal(t, "The childRepo into parentRepo AutoRoller is throttled", t1.msgs[2].subject)
	assert.Equal(t, fmt.Sprintf("The roller is throttled because it attempted to upload too many CLs in too short a time.  The roller will unthrottle at %s.", now.Format(time.RFC1123)), t1.msgs[2].m.Body)
	assert.Equal(t, notifier.SEVERITY_ERROR, t1.msgs[2].m.Severity)
}

func TestConfigs(t *testing.T) {
	testutils.SmallTest(t)

	c := Config{}
	_, _, _, err := c.notifier(nil)
	assert.EqualError(t, err, "\"type\" is required.")

	c = Config{
		"type": "bogus",
	}
	_, _, _, err = c.notifier(nil)
	assert.EqualError(t, err, "\"filter\" is required.")

	c = Config{
		"type":   "bogus",
		"filter": "bogus",
	}
	_, _, _, err = c.notifier(nil)
	assert.EqualError(t, err, "Unknown filter \"bogus\"")

	c = Config{
		"type":   "bogus",
		"filter": "debug",
	}
	_, _, _, err = c.notifier(nil)
	assert.EqualError(t, err, "Invalid notifier type \"bogus\"")

	c = Config{
		"type":   "email",
		"filter": "debug",
	}
	_, _, _, err = c.notifier(nil)
	assert.EqualError(t, err, "\"emails\" is required for type \"email\"")

	c = Config{
		"type":    "email",
		"filter":  "debug",
		"subject": "my subject",
		"emails":  []interface{}{"me@example.com"},
	}
	n, filter, subject, err := c.notifier(nil)
	assert.NoError(t, err)
	assert.NotNil(t, n)
	assert.Equal(t, filter, notifier.FILTER_DEBUG)
	assert.Equal(t, subject, "my subject")

	c = Config{
		"type":   "chat",
		"filter": "debug",
	}
	_, _, _, err = c.notifier(nil)
	assert.EqualError(t, err, "\"room\" is required.")

	c = Config{
		"type":    "chat",
		"filter":  "debug",
		"subject": "my subject",
		"room":    "my-room",
	}
	n, filter, subject, err = c.notifier(nil)
	assert.NoError(t, err)
	assert.NotNil(t, n)
	assert.Equal(t, filter, notifier.FILTER_DEBUG)
	assert.Equal(t, subject, "my subject")
}
