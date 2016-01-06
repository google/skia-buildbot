package buildbot

import (
	"bytes"
	"encoding/gob"
	"time"

	"go.skia.org/infra/go/buildbot/rpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// remoteDB is a struct used for interacting with a remote database.
type remoteDB struct {
	conn   *grpc.ClientConn
	client rpc.BuildbotDBClient
}

// NewRemoteDB returns a remote DB instance.
func NewRemoteDB(addr string) (DB, error) {
	// TODO(borenet): Shoudn't use WithInsecure...
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &remoteDB{
		conn:   conn,
		client: rpc.NewBuildbotDBClient(conn),
	}, nil
}

// Close closes the db.
func (d *remoteDB) Close() error {
	return d.conn.Close()
}

// See documentation for DB interface.
func (d *remoteDB) GetBuildsForCommits(commits []string, ignore map[string]bool) (map[string][]*Build, error) {
	ign := make([]string, 0, len(ignore))
	for i, _ := range ignore {
		ign = append(ign, i)
	}
	req := &rpc.GetBuildsForCommitsRequest{
		Commits: commits,
		Ignore:  ign,
	}
	resp, err := d.client.GetBuildsForCommits(context.Background(), req)
	if err != nil {
		return nil, err
	}
	rv := map[string][]*Build{}
	for k, v := range resp.Builds {
		builds := make([]*Build, 0, len(v.Builds))
		for _, build := range v.Builds {
			var b Build
			if err := gob.NewDecoder(bytes.NewBuffer(build.Build)).Decode(&b); err != nil {
				return nil, err
			}
			b.fixup()
			builds = append(builds, &b)
		}
		rv[k] = builds
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *remoteDB) GetBuild(id BuildID) (*Build, error) {
	req := &rpc.BuildID{
		Id: id,
	}
	resp, err := d.client.GetBuild(context.Background(), req)
	if err != nil {
		return nil, err
	}
	var b Build
	if err := gob.NewDecoder(bytes.NewBuffer(resp.Build)).Decode(&b); err != nil {
		return nil, err
	}
	b.fixup()
	return &b, nil
}

// See documentation for DB interface.
func (d *remoteDB) GetBuildFromDB(master, builder string, number int) (*Build, error) {
	return d.GetBuild(MakeBuildID(master, builder, number))
}

// See documentation for DB interface.
func (d *remoteDB) GetBuildsFromDateRange(start, end time.Time) ([]*Build, error) {
	req := &rpc.GetBuildsFromDateRangeRequest{
		Start: start.Format(time.RFC3339),
		End:   end.Format(time.RFC3339),
	}
	resp, err := d.client.GetBuildsFromDateRange(context.Background(), req)
	if err != nil {
		return nil, err
	}
	rv := make([]*Build, 0, len(resp.Builds))
	for _, build := range resp.Builds {
		var b Build
		if err := gob.NewDecoder(bytes.NewBuffer(build.Build)).Decode(&b); err != nil {
			return nil, err
		}
		b.fixup()
		rv = append(rv, &b)
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *remoteDB) GetBuildNumberForCommit(master, builder, commit string) (int, error) {
	req := &rpc.GetBuildNumberForCommitRequest{
		Master:  master,
		Builder: builder,
		Commit:  commit,
	}
	resp, err := d.client.GetBuildNumberForCommit(context.Background(), req)
	if err != nil {
		return -1, err
	}
	return int(resp.Val), nil
}

// See documentation for DB interface.
func (d *remoteDB) GetLastProcessedBuilds(m string) ([]BuildID, error) {
	req := &rpc.Master{
		Master: m,
	}
	resp, err := d.client.GetLastProcessedBuilds(context.Background(), req)
	if err != nil {
		return nil, err
	}
	rv := make([]BuildID, 0, len(resp.Ids))
	for _, id := range resp.Ids {
		rv = append(rv, id.Id)
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *remoteDB) GetMaxBuildNumber(master, builder string) (int, error) {
	req := &rpc.GetMaxBuildNumberRequest{
		Master:  master,
		Builder: builder,
	}
	resp, err := d.client.GetMaxBuildNumber(context.Background(), req)
	if err != nil {
		return -1, err
	}
	return int(resp.Val), nil
}

// See documentation for DB interface.
func (d *remoteDB) GetUnfinishedBuilds(m string) ([]*Build, error) {
	req := &rpc.Master{
		Master: m,
	}
	resp, err := d.client.GetUnfinishedBuilds(context.Background(), req)
	if err != nil {
		return nil, err
	}
	rv := make([]*Build, 0, len(resp.Builds))
	for _, build := range resp.Builds {
		var b Build
		if err := gob.NewDecoder(bytes.NewBuffer(build.Build)).Decode(&b); err != nil {
			return nil, err
		}
		b.fixup()
		rv = append(rv, &b)
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *remoteDB) PutBuild(b *Build) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(b); err != nil {
		return err
	}
	req := &rpc.Build{
		Build: buf.Bytes(),
	}
	if _, err := d.client.PutBuild(context.Background(), req); err != nil {
		return err
	}
	return nil
}

// See documentation for DB interface.
func (d *remoteDB) PutBuilds(builds []*Build) error {
	req := &rpc.PutBuildsRequest{
		Builds: make([]*rpc.Build, 0, len(builds)),
	}
	for _, b := range builds {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(b); err != nil {
			return err
		}
		req.Builds = append(req.Builds, &rpc.Build{
			Build: buf.Bytes(),
		})
	}
	if _, err := d.client.PutBuilds(context.Background(), req); err != nil {
		return err
	}
	return nil
}

// See documentation for DB interface.
func (d *remoteDB) NumIngestedBuilds() (int, error) {
	resp, err := d.client.NumIngestedBuilds(context.Background(), &rpc.Empty{})
	if err != nil {
		return -1, err
	}
	return int(resp.Ingested), nil
}

// See documentation for DB interface.
func (d *remoteDB) GetBuilderComments(builder string) ([]*BuilderComment, error) {
	req := &rpc.GetBuilderCommentsRequest{
		Builder: builder,
	}
	resp, err := d.client.GetBuilderComments(context.Background(), req)
	if err != nil {
		return nil, err
	}
	rv := make([]*BuilderComment, 0, len(resp.Comments))
	for _, comment := range resp.Comments {
		var c BuilderComment
		if err := gob.NewDecoder(bytes.NewBuffer(comment)).Decode(&c); err != nil {
			return nil, err
		}
		rv = append(rv, &c)
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *remoteDB) GetBuildersComments(builders []string) (map[string][]*BuilderComment, error) {
	req := &rpc.GetBuildersCommentsRequest{
		Builders: builders,
	}
	resp, err := d.client.GetBuildersComments(context.Background(), req)
	if err != nil {
		return nil, err
	}
	rv := map[string][]*BuilderComment{}
	for b, comments := range resp.Comments {
		bc := make([]*BuilderComment, 0, len(comments.Comments))
		for _, comment := range comments.Comments {
			var c BuilderComment
			if err := gob.NewDecoder(bytes.NewBuffer(comment)).Decode(&c); err != nil {
				return nil, err
			}
			bc = append(bc, &c)
		}
		rv[b] = bc
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *remoteDB) PutBuilderComment(c *BuilderComment) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(c); err != nil {
		return err
	}
	req := &rpc.PutBuilderCommentRequest{
		Comment: buf.Bytes(),
	}
	if _, err := d.client.PutBuilderComment(context.Background(), req); err != nil {
		return err
	}
	return nil
}

// See documentation for DB interface.
func (d *remoteDB) DeleteBuilderComment(id int64) error {
	req := &rpc.DeleteBuilderCommentRequest{
		Id: id,
	}
	if _, err := d.client.DeleteBuilderComment(context.Background(), req); err != nil {
		return err
	}
	return nil
}

// See documentation for DB interface.
func (d *remoteDB) GetCommitComments(commit string) ([]*CommitComment, error) {
	req := &rpc.GetCommitCommentsRequest{
		Commit: commit,
	}
	resp, err := d.client.GetCommitComments(context.Background(), req)
	if err != nil {
		return nil, err
	}
	rv := make([]*CommitComment, 0, len(resp.Comments))
	for _, comment := range resp.Comments {
		var c CommitComment
		if err := gob.NewDecoder(bytes.NewBuffer(comment)).Decode(&c); err != nil {
			return nil, err
		}
		rv = append(rv, &c)
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *remoteDB) GetCommitsComments(commits []string) (map[string][]*CommitComment, error) {
	req := &rpc.GetCommitsCommentsRequest{
		Commits: commits,
	}
	resp, err := d.client.GetCommitsComments(context.Background(), req)
	if err != nil {
		return nil, err
	}
	rv := map[string][]*CommitComment{}
	for commit, comments := range resp.Comments {
		cc := make([]*CommitComment, 0, len(comments.Comments))
		for _, comment := range comments.Comments {
			var c CommitComment
			if err := gob.NewDecoder(bytes.NewBuffer(comment)).Decode(&c); err != nil {
				return nil, err
			}
			cc = append(cc, &c)
		}
		rv[commit] = cc
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *remoteDB) PutCommitComment(c *CommitComment) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(c); err != nil {
		return err
	}
	req := &rpc.PutCommitCommentRequest{
		Comment: buf.Bytes(),
	}
	if _, err := d.client.PutCommitComment(context.Background(), req); err != nil {
		return err
	}
	return nil
}

// See documentation for DB interface.
func (d *remoteDB) DeleteCommitComment(id int64) error {
	req := &rpc.DeleteCommitCommentRequest{
		Id: id,
	}
	if _, err := d.client.DeleteCommitComment(context.Background(), req); err != nil {
		return err
	}
	return nil
}
