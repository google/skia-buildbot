package buildbot

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/buildbot/rpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func RunBuildServer(port string, db DB) (string, error) {
	if _, ok := db.(*localDB); !ok {
		return "", fmt.Errorf("Can only run a build server on a local DB instance.")
	}
	lis, err := net.Listen("tcp", port)
	if err != nil {
		return "", fmt.Errorf("Failed to create build server: failed to listen on port %q: %s", port, err)
	}
	s := grpc.NewServer()
	rpc.RegisterBuildbotDBServer(s, &rpcServer{db: db.(*localDB)})
	go func() {
		if err := s.Serve(lis); err != nil {
			glog.Errorf("Failed to run RPC server: %s", err)
		}
	}()
	addrSplit := strings.Split(lis.Addr().String(), ":")
	return fmt.Sprintf(":%s", addrSplit[len(addrSplit)-1]), nil
}

type rpcServer struct {
	db *localDB
}

func (s *rpcServer) BuildExists(ctx context.Context, req *rpc.GetBuildFromDBRequest) (*rpc.Bool, error) {
	rv, err := s.db.BuildExists(req.Master, req.Builder, int(req.Number))
	if err != nil {
		return nil, err
	}
	return &rpc.Bool{
		Val: rv,
	}, nil
}

func (s *rpcServer) GetBuildsForCommits(ctx context.Context, req *rpc.GetBuildsForCommitsRequest) (*rpc.GetBuildsForCommitsResponse, error) {
	ignore := map[string]bool{}
	for _, i := range req.Ignore {
		ignore[i] = true
	}
	builds, err := s.db.getBuildsForCommits(req.Commits, ignore, ctx.Done())
	if err != nil {
		return nil, err
	}
	rv := map[string]*rpc.Builds{}
	for k, v := range builds {
		buildsForCommit := make([]*rpc.Build, 0, len(v))
		for _, b := range v {
			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(b); err != nil {
				return nil, err
			}
			buildsForCommit = append(buildsForCommit, &rpc.Build{
				Build: buf.Bytes(),
			})
		}
		rv[k] = &rpc.Builds{
			Builds: buildsForCommit,
		}
	}
	return &rpc.GetBuildsForCommitsResponse{
		Builds: rv,
	}, nil
}

func (s *rpcServer) GetBuild(ctx context.Context, req *rpc.BuildID) (*rpc.Build, error) {
	build, err := s.db.GetBuild(req.Id)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(build); err != nil {
		return nil, err
	}
	return &rpc.Build{
		Build: buf.Bytes(),
	}, nil
}

func (s *rpcServer) GetBuildFromDB(ctx context.Context, req *rpc.GetBuildFromDBRequest) (*rpc.Build, error) {
	build, err := s.db.GetBuildFromDB(req.Master, req.Builder, int(req.Number))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(build); err != nil {
		return nil, err
	}
	return &rpc.Build{
		Build: buf.Bytes(),
	}, nil
}

func (s *rpcServer) GetBuildsFromDateRange(ctx context.Context, req *rpc.GetBuildsFromDateRangeRequest) (*rpc.Builds, error) {
	start, err := time.Parse(time.RFC3339, req.Start)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse(time.RFC3339, req.End)
	if err != nil {
		return nil, err
	}
	builds, err := s.db.getBuildsFromDateRange(start, end, ctx.Done())
	if err != nil {
		return nil, err
	}
	rv := &rpc.Builds{
		Builds: make([]*rpc.Build, 0, len(builds)),
	}
	for _, b := range builds {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(b); err != nil {
			return nil, err
		}
		rv.Builds = append(rv.Builds, &rpc.Build{
			Build: buf.Bytes(),
		})
	}
	return rv, nil
}

func (s *rpcServer) GetBuildNumberForCommit(ctx context.Context, req *rpc.GetBuildNumberForCommitRequest) (*rpc.Int64, error) {
	n, err := s.db.GetBuildNumberForCommit(req.Master, req.Builder, req.Commit)
	if err != nil {
		return nil, err
	}
	return &rpc.Int64{
		Val: int64(n),
	}, nil
}

func (s *rpcServer) GetLastProcessedBuilds(ctx context.Context, req *rpc.Master) (*rpc.BuildIDs, error) {
	ids, err := s.db.getLastProcessedBuilds(req.Master, ctx.Done())
	if err != nil {
		return nil, err
	}
	rv := &rpc.BuildIDs{
		Ids: make([]*rpc.BuildID, 0, len(ids)),
	}
	for _, id := range ids {
		rv.Ids = append(rv.Ids, &rpc.BuildID{
			Id: id,
		})
	}
	return rv, nil
}

func (s *rpcServer) GetMaxBuildNumber(ctx context.Context, req *rpc.GetMaxBuildNumberRequest) (*rpc.Int64, error) {
	n, err := s.db.GetMaxBuildNumber(req.Master, req.Builder)
	if err != nil {
		return nil, err
	}
	return &rpc.Int64{
		Val: int64(n),
	}, nil
}

func (s *rpcServer) GetModifiedBuilds(ctx context.Context, req *rpc.GetModifiedBuildsRequest) (*rpc.Builds, error) {
	builds, err := s.db.GetModifiedBuildsGOB(req.Id)
	if err != nil {
		return nil, err
	}
	rv := &rpc.Builds{
		Builds: make([]*rpc.Build, 0, len(builds)),
	}
	for _, b := range builds {
		rv.Builds = append(rv.Builds, &rpc.Build{
			Build: b.Bytes(),
		})
	}
	return rv, nil
}

func (s *rpcServer) StartTrackingModifiedBuilds(ctx context.Context, req *rpc.Empty) (*rpc.StartTrackingModifiedBuildsResponse, error) {
	id, err := s.db.StartTrackingModifiedBuilds()
	if err != nil {
		return nil, err
	}
	return &rpc.StartTrackingModifiedBuildsResponse{
		Id: id,
	}, nil
}

func (s *rpcServer) GetUnfinishedBuilds(ctx context.Context, req *rpc.Master) (*rpc.Builds, error) {
	builds, err := s.db.getUnfinishedBuilds(req.Master, ctx.Done())
	if err != nil {
		return nil, err
	}
	rv := &rpc.Builds{
		Builds: make([]*rpc.Build, 0, len(builds)),
	}
	for _, b := range builds {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(b); err != nil {
			return nil, err
		}
		rv.Builds = append(rv.Builds, &rpc.Build{
			Build: buf.Bytes(),
		})
	}
	return rv, nil
}

func (s *rpcServer) PutBuild(ctx context.Context, req *rpc.Build) (*rpc.Empty, error) {
	var b Build
	if err := gob.NewDecoder(bytes.NewBuffer(req.Build)).Decode(&b); err != nil {
		return nil, err
	}
	if err := s.db.PutBuild(&b); err != nil {
		return nil, err
	}
	return &rpc.Empty{}, nil
}

func (s *rpcServer) PutBuilds(ctx context.Context, req *rpc.PutBuildsRequest) (*rpc.Empty, error) {
	builds := make([]*Build, 0, len(req.Builds))
	for _, build := range req.Builds {
		var b Build
		if err := gob.NewDecoder(bytes.NewBuffer(build.Build)).Decode(&b); err != nil {
			return nil, err
		}
		builds = append(builds, &b)
	}
	if err := s.db.PutBuilds(builds); err != nil {
		return nil, err
	}
	return &rpc.Empty{}, nil
}

func (s *rpcServer) NumIngestedBuilds(ctx context.Context, req *rpc.Empty) (*rpc.NumIngestedBuildsResponse, error) {
	n, err := s.db.NumIngestedBuilds()
	if err != nil {
		return nil, err
	}
	return &rpc.NumIngestedBuildsResponse{
		Ingested: int64(n),
	}, nil
}

func (s *rpcServer) PutBuildComment(ctx context.Context, req *rpc.PutBuildCommentRequest) (*rpc.Empty, error) {
	var c BuildComment
	if err := gob.NewDecoder(bytes.NewBuffer(req.Comment)).Decode(&c); err != nil {
		return nil, err
	}
	if err := s.db.PutBuildComment(req.Master, req.Builder, int(req.Number), &c); err != nil {
		return nil, err
	}
	return &rpc.Empty{}, nil
}

func (s *rpcServer) DeleteBuildComment(ctx context.Context, req *rpc.DeleteBuildCommentRequest) (*rpc.Empty, error) {
	if err := s.db.DeleteBuildComment(req.Master, req.Builder, int(req.Number), req.Id); err != nil {
		return nil, err
	}
	return &rpc.Empty{}, nil
}

func (s *rpcServer) GetBuilderComments(ctx context.Context, req *rpc.GetBuilderCommentsRequest) (*rpc.GetBuilderCommentsResponse, error) {
	comments, err := s.db.GetBuilderComments(req.Builder)
	if err != nil {
		return nil, err
	}
	rv := make([][]byte, 0, len(comments))
	for _, c := range comments {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(&c); err != nil {
			return nil, err
		}
		rv = append(rv, buf.Bytes())
	}
	return &rpc.GetBuilderCommentsResponse{
		Comments: rv,
	}, nil
}

func (s *rpcServer) GetBuildersComments(ctx context.Context, req *rpc.GetBuildersCommentsRequest) (*rpc.GetBuildersCommentsResponse, error) {
	comments, err := s.db.GetBuildersComments(req.Builders)
	if err != nil {
		return nil, err
	}
	rv := map[string]*rpc.GetBuilderCommentsResponse{}
	for k, v := range comments {
		bc := make([][]byte, 0, len(v))
		for _, c := range v {
			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(&c); err != nil {
				return nil, err
			}
			bc = append(bc, buf.Bytes())
		}
		rv[k] = &rpc.GetBuilderCommentsResponse{
			Comments: bc,
		}
	}
	return &rpc.GetBuildersCommentsResponse{
		Comments: rv,
	}, nil
}

func (s *rpcServer) PutBuilderComment(ctx context.Context, req *rpc.PutBuilderCommentRequest) (*rpc.Empty, error) {
	var c BuilderComment
	if err := gob.NewDecoder(bytes.NewBuffer(req.Comment)).Decode(&c); err != nil {
		return nil, err
	}
	if err := s.db.PutBuilderComment(&c); err != nil {
		return nil, err
	}
	return &rpc.Empty{}, nil
}

func (s *rpcServer) DeleteBuilderComment(ctx context.Context, req *rpc.DeleteBuilderCommentRequest) (*rpc.Empty, error) {
	if err := s.db.DeleteBuilderComment(req.Id); err != nil {
		return nil, err
	}
	return &rpc.Empty{}, nil
}

func (s *rpcServer) GetCommitComments(ctx context.Context, req *rpc.GetCommitCommentsRequest) (*rpc.GetCommitCommentsResponse, error) {
	comments, err := s.db.GetCommitComments(req.Commit)
	if err != nil {
		return nil, err
	}
	rv := make([][]byte, 0, len(comments))
	for _, c := range comments {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(&c); err != nil {
			return nil, err
		}
		rv = append(rv, buf.Bytes())
	}
	return &rpc.GetCommitCommentsResponse{
		Comments: rv,
	}, nil
}

func (s *rpcServer) GetCommitsComments(ctx context.Context, req *rpc.GetCommitsCommentsRequest) (*rpc.GetCommitsCommentsResponse, error) {
	comments, err := s.db.GetCommitsComments(req.Commits)
	if err != nil {
		return nil, err
	}
	rv := map[string]*rpc.GetCommitCommentsResponse{}
	for k, v := range comments {
		cc := make([][]byte, 0, len(v))
		for _, c := range v {
			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(&c); err != nil {
				return nil, err
			}
			cc = append(cc, buf.Bytes())
		}
		rv[k] = &rpc.GetCommitCommentsResponse{
			Comments: cc,
		}
	}
	return &rpc.GetCommitsCommentsResponse{
		Comments: rv,
	}, nil
}

func (s *rpcServer) PutCommitComment(ctx context.Context, req *rpc.PutCommitCommentRequest) (*rpc.Empty, error) {
	var c CommitComment
	if err := gob.NewDecoder(bytes.NewBuffer(req.Comment)).Decode(&c); err != nil {
		return nil, err
	}
	if err := s.db.PutCommitComment(&c); err != nil {
		return nil, err
	}
	return &rpc.Empty{}, nil
}

func (s *rpcServer) DeleteCommitComment(ctx context.Context, req *rpc.DeleteCommitCommentRequest) (*rpc.Empty, error) {
	if err := s.db.DeleteCommitComment(req.Id); err != nil {
		return nil, err
	}
	return &rpc.Empty{}, nil
}
