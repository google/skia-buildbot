package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/auth"
	pb "go.skia.org/infra/go/coverage/proto/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
)

// getOauthCredentials returns a token source that will provide oauth tokens
// for the service account running the client process.
func getOauthCredentials(ctx context.Context) (oauth.TokenSource, error) {
	tokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	if err != nil {
		return oauth.TokenSource{}, skerr.Wrapf(err, "Failed to create oauth token source.")
	}
	return oauth.TokenSource{TokenSource: tokenSource}, nil
}

// getGrpcConnection returns a ClientConn object that can be used to create individual
// service clients for the coverage service.
func getGrpcConnection(host string, insecure_conn bool) *grpc.ClientConn {
	opts := []grpc.DialOption{}
	sklog.Debugf("Connecting to Host: %s", host)

	if insecure_conn {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsCreds := credentials.NewTLS(&tls.Config{
			// Since the communication is internal within the same GKE cluster,
			// we do not need to verify the server certificate.
			InsecureSkipVerify: true,
		})
		tokenSource, err := getOauthCredentials(context.Background())
		if err != nil {
			sklog.Errorf("Error Getting Token.")
			return nil

		}
		opts = append(opts, grpc.WithTransportCredentials(tlsCreds), grpc.WithPerRPCCredentials(tokenSource))
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
	ctx := context.Background()
	addBuilder := flag.Bool("addBuilder", false, "Add Builder to Database")
	addFile := flag.Bool("addFile", false, "Add File to Database")
	addTest := flag.Bool("addTest", false, "Add TestSuite to Database")
	deleteFile := flag.Bool("delete", false, "Remove File from Database")
	getAll := flag.Bool("getAll", false, "Get Full Database")
	host := flag.String("host", "localhost", "Hostname/IP of gRPC Service")
	port := flag.String("port", "8006", "Hostname/IP of gRPC Service")
	insecure := flag.Bool("insecure", true, "Use TLS secure connection")

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
	if *getAll {
		sampleFile = "getAll.json"
	}

	rpcHost := *host + ":" + *port
	conn := getGrpcConnection(rpcHost, *insecure)
	client := pb.NewCoverageServiceClient(conn)

	if strings.HasPrefix(sampleFile, "get") {
		if strings.HasPrefix(sampleFile, "getAll") {
			var request pb.CoverageRequest
			var response *pb.CoverageAllResponses
			err := json.Unmarshal(getJson(sampleFile), &request)
			if err != nil {
				sklog.Errorf("Unmarshal Error: %s", err)
				return
			}
			response, err = client.GetAllFiles(ctx, &request)
			sklog.Debugf(" Response: %s", response)
		} else {
			var listRequest pb.CoverageListRequest
			var listResponse *pb.CoverageListResponse
			err := json.Unmarshal(getJson(sampleFile), &listRequest)
			if err != nil {
				sklog.Errorf("Unmarshal Error: %s", err)
				return
			}
			listResponse, err = client.GetTestSuite(ctx, &listRequest)
			sklog.Debugf(" Response: %s", listResponse)
		}
	} else {
		var changeRequest pb.CoverageChangeRequest
		var changeResponse *pb.CoverageChangeResponse

		err := json.Unmarshal(getJson(sampleFile), &changeRequest)
		if err != nil {
			sklog.Errorf("Unmarshal Error: %s", err)
			return
		}

		if strings.HasPrefix(sampleFile, "add") {
			changeResponse, err = client.InsertFile(ctx, &changeRequest)
		} else {
			changeResponse, err = client.DeleteFile(ctx, &changeRequest)
		}
		if err != nil {
			sklog.Errorf("Error: %s", err.Error())
		} else {
			sklog.Debugf("Change Response: %s", changeResponse)
		}
	}
	conn.Close()
}
