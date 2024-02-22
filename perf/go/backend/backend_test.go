package backend

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func setupTestApp(t *testing.T) *Backend {
	flags := &config.BackendFlags{
		Port:     ":0",
		PromPort: ":0",
	}
	b, err := New(flags)
	require.NoError(t, err)
	ch := make(chan interface{})
	go func() {
		err := b.ServeGRPC()
		assert.NoError(t, err)
		ch <- nil
	}()

	t.Cleanup(func() {
		b.Cleanup()
		<-ch
	})

	return b
}

func TestAppSetup(t *testing.T) {
	b := setupTestApp(t)

	_, err := grpc.Dial(b.grpcPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
}
