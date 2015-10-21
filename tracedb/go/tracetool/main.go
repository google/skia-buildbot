// tracetool is a command-line tool for interrogating a tracedb server.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/trace/service"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// flags
var (
	address = flag.String("address", "localhost:9090", "The address of the traceservice gRPC endpoint.")
	begin   = flag.String("begin", "1w", "Select the commit ids for the range beginning this long ago.")
	end     = flag.String("end", "0s", "Select the commit ids for the range ending this long ago.")
)

var Usage = func() {
	fmt.Printf(`Usage: tracetool <command> [OPTIONS]...
Inspect and interrogate a running tracedb server.

Commands:

  ls        List all the commit ids for the given time range.

            Flags: --begin --end

  count     Return the number of samples stored for all commits in the given time range.

            Flags: --begin --end

  ping      Call the Ping service method every 1s.


Examples:

  To list all the commits for the first 6 days of the previous week:

    tracetool ls -begin 1w -end 1d

  To count all the values for every commit id in the last day:

    tracetool -begin 1d

Flags:

`)
	flag.PrintDefaults()
}

func _list(client traceservice.TraceServiceClient) (*traceservice.ListResponse, error) {
	req := &traceservice.ListRequest{}
	now := time.Now()
	b, err := human.ParseDuration(*begin)
	if err != nil {
		return nil, fmt.Errorf("Invalid begin value: %s", err)
	}
	e, err := human.ParseDuration(*end)
	if err != nil {
		return nil, fmt.Errorf("Invalid begin value: %s", err)
	}

	req.Begin = now.Add(-b).Unix()
	req.End = now.Add(-e).Unix()
	fmt.Printf("Requesting from %s to %s\n", now.Add(-b), now.Add(-e))
	return client.List(context.Background(), req)
}

func count(client traceservice.TraceServiceClient) {
	listResp, err := _list(client)
	if err != nil {
		fmt.Printf("Failed to retrieve the list: %s\n", err)
		return
	}
	fmt.Printf("Found %d commits.\n", len(listResp.Commitids))
	for _, cid := range listResp.Commitids {
		req := &traceservice.GetValuesRequest{
			Commitid: cid,
		}
		resp, err := client.GetValues(context.Background(), req)
		if err != nil {
			fmt.Printf("Failed to retrieve values: %s", err)
			return
		}
		fmt.Printf("%s  %s  %s: Count %d\n", cid.Id, cid.Source, time.Unix(cid.Timestamp, 0), len(resp.Values))
	}
}

func list(client traceservice.TraceServiceClient) {
	resp, err := _list(client)
	if err != nil {
		glog.Fatalf("Failed to retrieve the list: %s\n", err)
		return
	}
	for _, cid := range resp.Commitids {
		fmt.Printf("%s  %s  %s\n", cid.Id, cid.Source, time.Unix(cid.Timestamp, 0))
	}
}

func _pingStep(ctx context.Context, req *traceservice.Empty, client traceservice.TraceServiceClient) {
	begin := time.Now()
	_, err := client.Ping(ctx, req)
	end := time.Now()
	if err != nil {
		fmt.Printf("Failure: %s\n", err)
	} else {
		fmt.Printf("Success: time=%s\n", end.Sub(begin))
	}
}

func ping(client traceservice.TraceServiceClient) {
	ctx := context.Background()
	req := &traceservice.Empty{}
	_pingStep(ctx, req, client)
	for _ = range time.Tick(time.Second) {
		_pingStep(ctx, req, client)
	}
}

func main() {
	// Grab the first argument off of os.Args, the command, before we call flag.Parse.
	if len(os.Args) < 2 {
		Usage()
		return
	}
	cmd := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	// Now parge the flags.
	common.Init()

	// Set up a connection to the server.
	conn, err := grpc.Dial(*address, grpc.WithInsecure())
	if err != nil {
		glog.Fatalf("did not connect: %v", err)
	}
	defer util.Close(conn)
	client := traceservice.NewTraceServiceClient(conn)

	switch cmd {
	case "ls":
		list(client)
	case "ping":
		ping(client)
	case "count":
		count(client)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		Usage()
	}
}
