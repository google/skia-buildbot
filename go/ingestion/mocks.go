package ingestion

import (
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/sharedb"
	"go.skia.org/infra/go/trace/service"
	"go.skia.org/infra/go/vcsinfo"
)

type mockVCS []*vcsinfo.LongCommit

// MockVCS returns an instance of VCS that returns the commits passed as
// arguments.
func MockVCS(commits []*vcsinfo.LongCommit) vcsinfo.VCS {
	return mockVCS(commits)
}

func (m mockVCS) Update(pull, allBranches bool) error { return nil }
func (m mockVCS) From(start time.Time) []string {
	idx := sort.Search(len(m), func(i int) bool { return m[i].Timestamp.Unix() >= start.Unix() })

	ret := make([]string, 0, len(m)-idx)
	for _, commit := range m[idx:len(m)] {
		ret = append(ret, commit.Hash)
	}
	return ret
}

func (m mockVCS) Details(hash string, getBranches bool) (*vcsinfo.LongCommit, error) {
	for _, commit := range m {
		if commit.Hash == hash {
			return commit, nil
		}
	}
	return nil, fmt.Errorf("Unable to find commit")
}

// StartTestTraceDBServer starts up a traceDB server for testing. It stores its
// data at the given path and returns the address at which the server is
// listening as the second return value.
// Upon completion the calling test should call the Stop() function of the
// returned server object.
func StartTraceDBTestServer(t assert.TestingT, traceDBFileName, shareDBDir string) (*grpc.Server, string) {
	traceDBServer, err := traceservice.NewTraceServiceServer(traceDBFileName)
	assert.Nil(t, err)

	lis, err := net.Listen("tcp", "localhost:0")
	assert.Nil(t, err)

	server := grpc.NewServer()
	traceservice.RegisterTraceServiceServer(server, traceDBServer)

	if shareDBDir != "" {
		sharedb.RegisterShareDBServer(server, sharedb.NewServer(shareDBDir))
	}

	go func() {
		// We ignore the error, because calling the Stop() function always causes
		// an error and we are primarily interested in using this to test other code.
		_ = server.Serve(lis)
	}()

	return server, lis.Addr().String()
}
