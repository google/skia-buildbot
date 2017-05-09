package statistik

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/trace/service"
)

const (
	MAX_MESSAGE_SIZE = 1024 * 1024 * 1024
)

var (
	beginningOfTime = time.Date(2014, time.June, 18, 0, 0, 0, 0, time.UTC)
)

type DBBuilder struct {
	traceService traceservice.TraceServiceClient
}

func NewDBBuilder(addr string) (*DBBuilder, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithMaxMsgSize(MAX_MESSAGE_SIZE))
	if err != nil {
		return nil, fmt.Errorf("Unable to connnect to trace service at %s. Got error: %s", addr, err)
	}

	return &DBBuilder{
		traceService: traceservice.NewTraceServiceClient(conn),
	}, nil
}

func (d *DBBuilder) ExtractTest(name string, startTime, endTime time.Time) (*tiling.Tile, error) {
	// resp, err := d.traceService.List(ctx, req)
	// if err != nil {
	// 	return nil, err
	// }

	// Iterate over everything we have and build a tile for just that test.
	listReq := &traceservice.ListRequest{
		Begin: startTime.Unix(),
		End:   endTime.Unix(),
	}
	ctx := context.Background()
	listResp, err := d.traceService.List(ctx, listReq)
	if err != nil {
		return nil, fmt.Errorf("List request failed: %s", err)
	}

	// Copy the data from the ListResponse to a slice of CommitIDs.
	ret := []*traceservice.CommitID{}
	for _, c := range listResp.Commitids {
		ret = append(ret, &traceservice.CommitID{
			Id:        c.Id,
			Source:    c.Source,
			Timestamp: c.Timestamp,
		})
	}

	gpReq := traceservice.GetParamsRequest{}
	d.traceService.GetParams(ctx, gpReq)

	return nil, nil
}
