package notifier

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

type sentMessage struct {
	subject string
	msg     *Message
}

type testNotifier struct {
	sent []*sentMessage
}

func (n *testNotifier) Send(_ context.Context, subject string, msg *Message) error {
	n.sent = append(n.sent, &sentMessage{
		subject: subject,
		msg:     msg,
	})
	return nil
}

func TestRouter(t *testing.T) {
	testutils.SmallTest(t)

	m := NewRouter(nil, nil, nil)
	ctx := context.Background()

	n1 := &testNotifier{}
	m.Add(n1, FILTER_DEBUG, nil, "")
	n2 := &testNotifier{}
	m.Add(n2, FILTER_WARNING, nil, "")
	n3 := &testNotifier{}
	m.Add(n3, Filter(0), []string{"whitelisted type"}, "")

	assert.NoError(t, m.Send(ctx, &Message{
		Subject:  "Hi!",
		Body:     "Message body",
		Severity: SEVERITY_INFO,
		Type:     "my-msg-type",
	}))

	assert.Equal(t, 1, len(n1.sent))
	assert.Equal(t, "Hi!", n1.sent[0].subject)
	assert.Equal(t, "Message body", n1.sent[0].msg.Body)
	assert.Equal(t, 0, len(n2.sent))
	assert.Equal(t, 0, len(n3.sent))

	n4 := &testNotifier{}
	m.Add(n4, FILTER_INFO, nil, "One subject to rule them all")

	assert.NoError(t, m.Send(ctx, &Message{
		Subject:  "My subject",
		Body:     "Second Message",
		Severity: SEVERITY_ERROR,
		Type:     "whitelisted type",
	}))

	assert.Equal(t, 1, len(n4.sent))
	assert.Equal(t, "One subject to rule them all", n4.sent[0].subject)
	assert.Equal(t, "Second Message", n4.sent[0].msg.Body)
	assert.Equal(t, 1, len(n3.sent))
	assert.Equal(t, "My subject", n3.sent[0].subject)
	assert.Equal(t, "Second Message", n3.sent[0].msg.Body)
}
