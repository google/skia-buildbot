package ingestion

import (
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/sharedb"
	"go.skia.org/infra/go/trace/service"
	"go.skia.org/infra/go/vcsinfo"
)

type MockVCSImpl struct {
	commits            []*vcsinfo.LongCommit
	depsFileMap        map[string]string
	secondaryVCS       vcsinfo.VCS
	secondaryExtractor depot_tools.DEPSExtractor
}

// MockVCS returns an instance of VCS that returns the commits passed as
// arguments.
func MockVCS(commits []*vcsinfo.LongCommit, depsContentMap map[string]string) vcsinfo.VCS {
	return MockVCSImpl{
		commits:     commits,
		depsFileMap: depsContentMap,
	}
}

func (m MockVCSImpl) Update(pull, allBranches bool) error               { return nil }
func (m MockVCSImpl) LastNIndex(N int) []*vcsinfo.IndexCommit           { return nil }
func (m MockVCSImpl) Range(begin, end time.Time) []*vcsinfo.IndexCommit { return nil }
func (m MockVCSImpl) IndexOf(hash string) (int, error) {
	return 0, nil
}
func (m MockVCSImpl) From(start time.Time) []string {
	idx := sort.Search(len(m.commits), func(i int) bool { return m.commits[i].Timestamp.Unix() >= start.Unix() })

	ret := make([]string, 0, len(m.commits)-idx)
	for _, commit := range m.commits[idx:] {
		ret = append(ret, commit.Hash)
	}
	return ret
}

func (m MockVCSImpl) Details(hash string, getBranches bool) (*vcsinfo.LongCommit, error) {
	for _, commit := range m.commits {
		if commit.Hash == hash {
			return commit, nil
		}
	}
	return nil, fmt.Errorf("Unable to find commit")
}

func (m MockVCSImpl) ByIndex(N int) (*vcsinfo.LongCommit, error) {
	return nil, nil
}

func (m MockVCSImpl) GetFile(fileName, commitHash string) (string, error) {
	return m.depsFileMap[commitHash], nil
}

func (m MockVCSImpl) ResolveCommit(commitHash string) (string, error) {
	if m.secondaryVCS == nil {
		return "", nil
	}
	foundCommit, err := m.secondaryExtractor.ExtractCommit(m.secondaryVCS.GetFile("DEPS", commitHash))
	if err != nil {
		return "", err
	}
	return foundCommit, nil
}

func (m MockVCSImpl) SetSecondaryRepo(secVCS vcsinfo.VCS, extractor depot_tools.DEPSExtractor) {
	m.secondaryVCS = secVCS
	m.secondaryExtractor = extractor
}

// StartTestTraceDBServer starts up a traceDB server for testing. It stores its
// data at the given path and returns the address at which the server is
// listening as the second return value.
// Upon completion the calling test should call the Stop() function of the
// returned server object.
func StartTraceDBTestServer(t assert.TestingT, traceDBFileName, shareDBDir string) (*grpc.Server, string) {
	traceDBServer, err := traceservice.NewTraceServiceServer(traceDBFileName)
	assert.NoError(t, err)

	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)

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
