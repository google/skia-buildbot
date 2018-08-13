package streamer

import (
	"io"

	"go.skia.org/infra/go/sklog"
)

// Generate the go code from the protocol buffer definitions.
// //  go:generate protoc --go_out=plugins=grpc:. pktservice.proto
//go:generate protoc --go_out=plugins=grpc:. streamer.proto

type serverImpl struct {
}

func NewServer() MsgServiceServer {
	return &serverImpl{}
}

func (s *serverImpl) Ops(stream MsgService_OpsServer) error {
	// TODO: Add this stream to the client identified by meta data.

	for {
		funcMsg, err := stream.Recv()
		if err == io.EOF {
			// TODO: handle the client closing the connection.
			return nil
		}
		if err != nil {
			return err
		}

		sklog.Infof("Payload: %s", funcMsg.Payload)
		// TODO: Get the func response and send it off to be matche with
		// the corresponding request.
	}
}

func (s *serverImpl) Status(stream MsgService_StatusServer) error {
	// TODO: Create a new client or update an existing client with
	// this stream.

	for {
		statusMsg, err := stream.Recv()
		if err == io.EOF {
			// TODO: handle the client closing the connection.
			// probably by removing this stream from the client record.
			return nil
		}
		if err != nil {
			return err
		}
		sklog.Infof("%d", statusMsg.TimeStamp)
		// TODO: Get the status information from the client and handle it.
	}
}
