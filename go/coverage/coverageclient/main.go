package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	pb "go.skia.org/infra/go/coverage/proto/v1"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// getGrpcConnection returns a ClientConn object that can be used to create individual
// service clients for the coverage service.
func getGrpcConnection(host string, insecure_conn bool) *grpc.ClientConn {
	opts := []grpc.DialOption{}
	sklog.Debugf("Connecting to Host: %s", host)

	if insecure_conn {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(host, opts...)
	if err != nil {
		sklog.Errorf("Error connecting to Coverage service at %s: %s", host, err)
		return nil
	}
	return conn
}

func getJson(filename string) []byte {
	sklog.Debugf("Loading File: %s", filename)
	cwd, err := os.Getwd()
	file := filepath.Join(cwd, "coverageclient", filename)
	jsonFile, err := os.Open(file)
	if err != nil {
		sklog.Errorf("Error loading Json %s:", err)
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
	return byteValue
}

func main() {
	var request pb.CoverageRequest
	var response *pb.CoverageChangeResponse

	ctx := context.Background()
	addBuilder := flag.Bool("addBuilder", false, "Add Builder to Database")
	addFile := flag.Bool("addFile", false, "Add File to Database")
	addTest := flag.Bool("addTest", false, "Add TestSuite to Database")
	deleteFile := flag.Bool("delete", false, "Remove File from Database")
	host := flag.String("host", "localhost", "Hostname/IP of gRPC Service")
	port := flag.String("port", "8006", "Hostname/IP of gRPC Service")

	flag.Parse()

	sampleFile := "getTestSuite.json"
	if *addFile {
		sampleFile = "addFile.json"
	}
	if *addBuilder {
		sampleFile = "addBuilder.json"
	}
	if *addTest {
		sampleFile = "addTestSuite.json"
	}
	if *deleteFile {
		sampleFile = "deleteFile.json"
	}

	err := json.Unmarshal(getJson(sampleFile), &request)
	if err != nil {
		sklog.Errorf("Unmarshal Error: %s", err)
		return
	}

	rpcHost := *host + ":" + *port
	conn := getGrpcConnection(rpcHost, true)
	client := pb.NewCoverageServiceClient(conn)

	if strings.HasPrefix(sampleFile, "get") {
		var listResponse *pb.CoverageListResponse
		listResponse, err = client.GetTestSuite(ctx, &request)
		sklog.Debugf(" Response: %s", listResponse)
	} else {
		if strings.HasPrefix(sampleFile, "add") {
			response, err = client.InsertFile(ctx, &request)
		} else {
			response, err = client.DeleteFile(ctx, &request)
		}
		if err != nil {
			sklog.Errorf("Error: %s", err.Error())
		} else {
			sklog.Debugf("Change Response: %s", response)
		}
	}
	conn.Close()
}
