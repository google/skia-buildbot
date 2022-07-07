package notifier

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/email/go/emailclient"
	"go.skia.org/infra/go/notifier"
	"go.skia.org/infra/go/testutils/unittest"
)

type msg struct {
	subject string
	m       *notifier.Message
}

type testNotifier struct {
	msgs []*msg
}

func (n *testNotifier) Send(ctx context.Context, subject string, m *notifier.Message) error {
	n.msgs = append(n.msgs, &msg{
		subject: subject,
		m:       m,
	})
	return nil
}

func TestNotifier(t *testing.T) {
	unittest.SmallTest(t)

	ctx := context.Background()
	n, err := New(ctx, "childRepo", "parentRepo", "https://autoroll.skia.org/r/test-roller", nil, emailclient.New(), nil, nil)
	require.NoError(t, err)
	t1 := &testNotifier{}
	n.Router().Add(t1, notifier.FILTER_DEBUG, nil, "")
	t2 := &testNotifier{}
	n.Router().Add(t2, notifier.FILTER_SILENT, []string{MSG_TYPE_MODE_CHANGE}, "")

	footer := "\n\nThe AutoRoll server is located here: https://autoroll.skia.org/r/test-roller"
	n.SendIssueUpdate(ctx, "123", "https://codereview/123", "uploaded a CL!")
	require.Equal(t, 1, len(t1.msgs))
	require.Equal(t, "The childRepo into parentRepo AutoRoller has uploaded issue 123", t1.msgs[0].subject)
	require.Equal(t, "uploaded a CL!"+footer, t1.msgs[0].m.Body)
	require.Equal(t, notifier.SEVERITY_INFO, t1.msgs[0].m.Severity)
	require.Equal(t, 0, len(t2.msgs))

	n.SendModeChange(ctx, "test@skia.org", "STOPPED", "<b>Staaahhp!</b>")
	require.Equal(t, 2, len(t1.msgs))
	require.Equal(t, "The childRepo into parentRepo AutoRoller mode was changed", t1.msgs[1].subject)
	require.Equal(t, "test@skia.org changed the mode to \"STOPPED\" with message: &lt;b&gt;Staaahhp!&lt;/b&gt;"+footer, t1.msgs[1].m.Body)
	require.Equal(t, notifier.SEVERITY_WARNING, t1.msgs[1].m.Severity)
	require.Equal(t, 1, len(t2.msgs))
	require.Equal(t, "The childRepo into parentRepo AutoRoller mode was changed", t2.msgs[0].subject)
	require.Equal(t, "test@skia.org changed the mode to \"STOPPED\" with message: &lt;b&gt;Staaahhp!&lt;/b&gt;"+footer, t2.msgs[0].m.Body)
	require.Equal(t, notifier.SEVERITY_WARNING, t2.msgs[0].m.Severity)

	now := time.Now().Round(time.Millisecond)
	n.SendSafetyThrottled(ctx, now)
	require.Equal(t, 3, len(t1.msgs))
	require.Equal(t, "The childRepo into parentRepo AutoRoller is throttled", t1.msgs[2].subject)
	require.Equal(t, fmt.Sprintf("The roller is throttled because it attempted to upload too many CLs in too short a time.  The roller will unthrottle at %s."+footer, now.Format(time.RFC1123)), t1.msgs[2].m.Body)
	require.Equal(t, notifier.SEVERITY_ERROR, t1.msgs[2].m.Severity)
	require.Equal(t, 1, len(t2.msgs))
}
