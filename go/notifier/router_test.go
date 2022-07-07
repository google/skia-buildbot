package notifier

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/email/go/emailclient"
	"go.skia.org/infra/go/testutils/unittest"
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
	unittest.SmallTest(t)

	m := NewRouter(nil, emailclient.New(), nil)
	ctx := context.Background()

	n1 := &testNotifier{}
	m.Add(n1, FILTER_DEBUG, nil, "")
	n2 := &testNotifier{}
	m.Add(n2, FILTER_WARNING, nil, "")
	n3 := &testNotifier{}
	m.Add(n3, Filter(0), []string{"included type"}, "")

	require.NoError(t, m.Send(ctx, &Message{
		Subject:  "Hi!",
		Body:     "Message body",
		Severity: SEVERITY_INFO,
		Type:     "my-msg-type",
	}))

	require.Equal(t, 1, len(n1.sent))
	require.Equal(t, "Hi!", n1.sent[0].subject)
	require.Equal(t, "Message body", n1.sent[0].msg.Body)
	require.Equal(t, 0, len(n2.sent))
	require.Equal(t, 0, len(n3.sent))

	n4 := &testNotifier{}
	m.Add(n4, FILTER_INFO, nil, "One subject to rule them all")

	require.NoError(t, m.Send(ctx, &Message{
		Subject:  "My subject",
		Body:     "Second Message",
		Severity: SEVERITY_ERROR,
		Type:     "included type",
	}))

	require.Equal(t, 1, len(n4.sent))
	require.Equal(t, "One subject to rule them all", n4.sent[0].subject)
	require.Equal(t, "Second Message", n4.sent[0].msg.Body)
	require.Equal(t, 1, len(n3.sent))
	require.Equal(t, "My subject", n3.sent[0].subject)
	require.Equal(t, "Second Message", n3.sent[0].msg.Body)
}
