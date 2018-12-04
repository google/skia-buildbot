// tracetool is a command-line tool for interrogating a tracedb server.
package main

import (
	"context"
	"crypto/md5"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/sklog"
	traceservice "go.skia.org/infra/go/trace/service"
	"go.skia.org/infra/go/util"
	"google.golang.org/grpc"
)

// flags
var (
	address = flag.String("address", "localhost:9090", "The address of the traceservice gRPC endpoint.")
	begin   = flag.String("begin", "1w", "Select the commit ids for the range beginning this long ago.")
	end     = flag.String("end", "0s", "Select the commit ids for the range ending this long ago.")
	id      = flag.String("id", "", "Selects the CommitID with an ID that begins with id.")
	regex   = flag.String("regex", "", "A regular expression to match against traceids.")
	verbose = flag.Bool("verbose", false, "Verbose output.")
	only    = flag.Bool("only", false, "If true then only print values, otherwise print keys and values.")
	showMD5 = flag.Bool("md5", false, "If true then include the MD5 hash value for each commit id to compare across databases. Warning: Slow !")
)

var Usage = func() {
	fmt.Printf(`Usage: tracetool <command> [OPTIONS]...
Inspect and interrogate a running tracedb server.

Commands:

  ls        	List all the commit ids for the given time range.

            	Flags: --begin --end --md5

  count     	Return the number of samples stored for all commits in the given time range.

            	Flags: --begin --end

  ping      	Call the Ping service method every 1s.

  sample    	Get a sampling of values for the given ID.
            	Flags: --begin --end --id --regex --only

            	The first commitid with an ID that begins with the value of --id
            	will be loaded and a sampling of 10 values will be displayed.

  param_grep  Find parameter values that match a regular expression.
  						Flags: --begin --end --regex

	  					It loads all commits in the defined range and matches the
	  					parameter values of each trace in those commits against the
	  					regular expression. For each commit it outputs the paramaters
	  					and their values that match the regex.

  value_grep  Find commits where at least one value matches a regular expression.
  						Flags: --begin --end --regex --verbose

	  					It loads all commits in the defined range and matches the
	  					values of each trace in those commits against the
	  					regular expression. For each commit it outputs the values of the
	  					"name" parameter across traces.
	  					This only makes sense for Gold data since the digests are stored
	  					strings.

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
	if *verbose {
		fmt.Printf("Requesting from %s to %s\n", now.Add(-b), now.Add(-e))
	}
	return client.List(context.Background(), req)
}

func count(client traceservice.TraceServiceClient) {
	listResp, err := _list(client)
	if err != nil {
		fmt.Printf("Failed to retrieve the list: %s\n", err)
		return
	}
	for _, cid := range listResp.Commitids {
		if *id != "" && !strings.HasPrefix(cid.Id, *id) {
			continue
		}
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
		sklog.Fatalf("Failed to retrieve the list: %s\n", err)
	}

	for _, cid := range resp.Commitids {
		if *id != "" && !strings.HasPrefix(cid.Id, *id) {
			continue
		}
		fmt.Printf("%s  %s  %s", cid.Id, cid.Source, time.Unix(cid.Timestamp, 0).UTC())
		if *showMD5 {
			fmt.Printf(" %s", calcMD5FromValues(client, cid))
		}
		fmt.Printf("\n")
	}
}

// calcMD5FromValues loads the values for the given commit ID and sorts the bytes of the
// values in lexicographical order and calculates the hash. This can be used to compare
// commit values across databases.
func calcMD5FromValues(client traceservice.TraceServiceClient, cid *traceservice.CommitID) string {
	req := &traceservice.GetValuesRequest{Commitid: cid}
	resp, err := client.GetValues(context.Background(), req)
	if err != nil {
		sklog.Fatalf("Failed to retrieve value: %s", err)
	}

	sortedStr := make([]string, 0, len(resp.Values))
	for _, val := range resp.Values {
		sortedStr = append(sortedStr, string(val.Value))
	}
	sort.Strings(sortedStr)

	ret := md5.New()
	for _, val := range sortedStr {
		_, err := ret.Write([]byte(val))
		if err != nil {
			sklog.Fatalf("Error writint to MD5: %s", err)
		}
	}
	return fmt.Sprintf("%x", ret.Sum(nil))
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
	for range time.Tick(time.Second) {
		_pingStep(ctx, req, client)
	}
}

// converter is a func that will convert the raw byte slice returned into the correct type.
type converter func([]byte) interface{}

// goldConverter in an implementation of converter for digests (strings).
func goldConverter(b []byte) interface{} {
	return string(b)
}

func sample(client traceservice.TraceServiceClient) {
	// Get all the CommitIDs in the given time range.
	listResp, err := _list(client)
	if err != nil {
		fmt.Printf("Failed to retrieve the list: %s\n", err)
		return
	}

	for _, cid := range listResp.Commitids {
		if *id != "" && !strings.HasPrefix(cid.Id, *id) {
			continue
		}
		req := &traceservice.GetValuesRequest{
			Commitid: cid,
		}
		resp, err := client.GetValues(context.Background(), req)
		if err != nil {
			fmt.Printf("Failed to retrieve values: %s", err)
			return
		}
		if *regex == "" {
			// Dump a sample of at most 10 values along with their traceids.
			N := 10
			if len(resp.Values) < N {
				N = len(resp.Values)
			}
			for i := 0; i < N; i++ {
				pair := resp.Values[rand.Intn(len(resp.Values))]
				if *only {
					fmt.Printf("%v\n", goldConverter(pair.Value))
				} else {
					fmt.Printf("%110s  -  %v\n", pair.Key, goldConverter(pair.Value))
				}
			}
		} else {
			r, err := regexp.Compile(*regex)
			if err != nil {
				fmt.Printf("Invalid value for regex %q: %s\n", *regex, err)
			}
			for _, pair := range resp.Values {
				if r.MatchString(pair.Key) {
					if *only {
						fmt.Printf("%v\n", goldConverter(pair.Value))
					} else {
						fmt.Printf("%110s  -  %v\n", pair.Key, goldConverter(pair.Value))
					}
				}
			}
		}
	}
}

func param_grep(client traceservice.TraceServiceClient) {
	if *regex == "" {
		sklog.Fatalf("No regex given for param_grep")
	}
	r, err := regexp.Compile(*regex)
	if err != nil {
		sklog.Fatalf("Invalid value for regex %q: %s\n", *regex, err)
	}

	ctx := context.Background()
	resp, err := _list(client)
	if err != nil {
		sklog.Fatalf("Failed to retrieve the list: %s\n", err)
	}

	for _, cid := range resp.Commitids {
		traceIdsResp, err := client.GetValues(ctx, &traceservice.GetValuesRequest{Commitid: cid})
		if err != nil {
			sklog.Errorf("Could not get trace ids: %s", err)
			continue
		}

		traceIds := make([]string, 0, len(traceIdsResp.Values))
		for _, valuePair := range traceIdsResp.Values {
			traceIds = append(traceIds, valuePair.Key)
		}

		paramsResp, err := client.GetParams(ctx, &traceservice.GetParamsRequest{Traceids: traceIds})
		if err != nil {
			sklog.Errorf("Unable to retrieve params for %s. Error: %s", cid, err)
			continue
		}

		result := make(map[string]bool, len(paramsResp.Params))
		for _, p := range paramsResp.Params {
			for key, val := range p.Params {
				if r.MatchString(val) {
					result[fmt.Sprintf("%s = %s", key, val)] = true
				}
			}
		}

		fmt.Printf("%.3d %s  %s  %s \n", len(result), cid.Id, cid.Source, time.Unix(cid.Timestamp, 0))
		for p := range result {
			fmt.Printf("       %s\n", p)
		}
	}
}

func value_grep(client traceservice.TraceServiceClient) {
	if *regex == "" {
		sklog.Fatalf("No regex given for param_grep")
	}
	r, err := regexp.Compile(*regex)
	if err != nil {
		sklog.Fatalf("Invalid value for regex %q: %s\n", *regex, err)
	}

	ctx := context.Background()
	resp, err := _list(client)
	if err != nil {
		sklog.Fatalf("Failed to retrieve the list: %s\n", err)
	}

	for _, cid := range resp.Commitids {
		traceIdsResp, err := client.GetValues(ctx, &traceservice.GetValuesRequest{Commitid: cid})
		if err != nil {
			sklog.Errorf("Could not get trace ids: %s", err)
			continue
		}

		traceIds := make([]string, 0, len(traceIdsResp.Values))
		for _, valuePair := range traceIdsResp.Values {
			if r.MatchString(string(valuePair.Value)) {
				traceIds = append(traceIds, valuePair.Key)
			}
		}

		if len(traceIds) == 0 {
			if *verbose {
				fmt.Printf("NOT FOUND IN %s  %s  %s\n", cid.Id, cid.Source, time.Unix(cid.Timestamp, 0))
			}
			continue
		}

		paramsResp, err := client.GetParams(ctx, &traceservice.GetParamsRequest{Traceids: traceIds})
		if err != nil {
			sklog.Errorf("Unable to retrieve params for %s. Error: %s", cid, err)
			continue
		}

		result := map[string]bool{}
		for _, p := range paramsResp.Params {
			if name, ok := p.Params["name"]; ok {
				result[name] = true
			}
		}

		fmt.Printf("%.3d %s  %s  %s : ", len(result), cid.Id, cid.Source, time.Unix(cid.Timestamp, 0))
		for name := range result {
			fmt.Print(name + "  ")
		}
		fmt.Println()
	}
}

func main() {
	rand.Seed(time.Now().Unix())
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
		sklog.Fatalf("did not connect: %v", err)
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
	case "sample":
		sample(client)
	case "param_grep":
		param_grep(client)
	case "value_grep":
		value_grep(client)
	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		Usage()
	}
}
