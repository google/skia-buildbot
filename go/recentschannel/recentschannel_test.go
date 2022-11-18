package recentschannel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSend_DoesNotBlockAndKeepsMostRecent(t *testing.T) {
	c := New[int](2)
	c.Send(2)
	c.Send(3)
	c.Send(4) // If it blocks on send, it will never get past here.
	c.Send(5)
	out := c.Recv()
	require.Equal(t, 4, <-out)
	require.Equal(t, 5, <-out)
}
