package streamer

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/util"
)

type AsyncOp struct {
	ID        int64
	ReqCodec  util.LRUCodec
	RespCodec util.LRUCodec
}

const (
	restart_OPID = iota
)

func TestMsgService(t *testing.T) {
	// Create a server and start it

	serverOps := []AsyncOp{}

	// Create a client and connect to the server
	serverAddr := ""
	server, err := NewStreamerServer(serverAddr, serverOps)

	client, err := NewStreamerClient(serverAddr)
	assert.NoError(t, err)

	server.ExecOp(opID, req, clientIDs)

	assert.Fail(t, "test failure")
}
