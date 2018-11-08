package firestore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	COLLECTION_COMMIT_COMMENTS    = "commit-comments"
	COLLECTION_TASK_COMMENTS      = "task-comments"
	COLLECTION_TASK_SPEC_COMMENTS = "task-spec-comments"
)

// commitComments returns a reference to the commit comments collection.
func (d *firestoreDB) commitComments() *firestore.CollectionRef {
	return d.collection(COLLECTION_COMMIT_COMMENTS)
}

// taskComments returns a reference to the task comments collection.
func (d *firestoreDB) taskComments() *firestore.CollectionRef {
	return d.collection(COLLECTION_TASK_COMMENTS)
}

// taskSpecComments returns a reference to the task spec comments collection.
func (d *firestoreDB) taskSpecComments() *firestore.CollectionRef {
	return d.collection(COLLECTION_TASK_SPEC_COMMENTS)
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) GetCommentsForRepos(repos []string, from time.Time) ([]*db.RepoComments, error) {
	from = fixTimestamp(from)
	// TODO(borenet): We should come up with something more efficient, but
	// this is convenient because it doesn't require composite indexes.
	commentsByRepo := make(map[string]*db.RepoComments, len(repos))
	for _, repo := range repos {
		commentsByRepo[repo] = &db.RepoComments{
			Repo: repo,
		}
	}

	iter := func(cr *firestore.CollectionRef, newObj func() interface{}, cb func(interface{})) error {
		it := cr.Where("Timestamp", ">=", from).OrderBy("Timestamp", firestore.Asc).Documents(context.Background())
		for {
			doc, err := it.Next()
			if err == iterator.Done {
				break
			} else if err != nil {
				return fmt.Errorf("Iteration failed: %s", err)
			}
			obj := newObj()
			if err := doc.DataTo(obj); err != nil {
				return fmt.Errorf("Failed to copy data: %s", err)
			}
			cb(obj)
		}
		return nil
	}

	if err := iter(d.commitComments(), func() interface{} {
		return &db.CommitComment{}
	}, func(iface interface{}) {
		c := iface.(*db.CommitComment)
		if comments, ok := commentsByRepo[c.Repo]; ok {
			if comments.CommitComments == nil {
				comments.CommitComments = map[string][]*db.CommitComment{}
			}
			comments.CommitComments[c.Revision] = append(comments.CommitComments[c.Revision], c)
		}
	}); err != nil {
		return nil, err
	}

	if err := iter(d.taskComments(), func() interface{} {
		return &db.TaskComment{}
	}, func(iface interface{}) {
		c := iface.(*db.TaskComment)
		if comments, ok := commentsByRepo[c.Repo]; ok {
			if comments.TaskComments == nil {
				comments.TaskComments = map[string]map[string][]*db.TaskComment{}
			}
			byCommit, ok := comments.TaskComments[c.Revision]
			if !ok {
				byCommit = map[string][]*db.TaskComment{}
				comments.TaskComments[c.Revision] = byCommit
			}
			byCommit[c.Name] = append(byCommit[c.Name], c)
		}
	}); err != nil {
		return nil, err
	}

	if err := iter(d.taskSpecComments(), func() interface{} {
		return &db.TaskSpecComment{}
	}, func(iface interface{}) {
		c := iface.(*db.TaskSpecComment)
		if comments, ok := commentsByRepo[c.Repo]; ok {
			if comments.TaskSpecComments == nil {
				comments.TaskSpecComments = map[string][]*db.TaskSpecComment{}
			}
			comments.TaskSpecComments[c.Name] = append(comments.TaskSpecComments[c.Name], c)
		}
	}); err != nil {
		return nil, err
	}

	rv := make([]*db.RepoComments, 0, len(repos))
	for _, repo := range repos {
		rv = append(rv, commentsByRepo[repo])
	}
	return rv, nil
}

// taskCommentId returns an ID for the TaskComment.
func taskCommentId(c *db.TaskComment) string {
	// TODO(borenet): Use Firestore-assigned IDs.
	return fmt.Sprintf("%s#%s#%s#%s", c.Repo, c.Revision, c.Name, fixTimestamp(c.Timestamp).Format(local_db.TIMESTAMP_FORMAT))
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) PutTaskComment(c *db.TaskComment) error {
	c.Timestamp = fixTimestamp(c.Timestamp)
	id := taskCommentId(c)
	_, err := d.taskComments().Doc(id).Create(context.Background(), c)
	if grpc.Code(err) == codes.AlreadyExists {
		return db.ErrAlreadyExists
	}
	return err
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) DeleteTaskComment(c *db.TaskComment) error {
	id := taskCommentId(c)
	_, err := d.taskComments().Doc(id).Delete(context.Background())
	return err
}

// taskSpecCommentId returns an ID for the TaskSpecComment.
func taskSpecCommentId(c *db.TaskSpecComment) string {
	// TODO(borenet): Use Firestore-assigned IDs.
	return fmt.Sprintf("%s#%s#%s", c.Repo, c.Name, c.Timestamp.Format(local_db.TIMESTAMP_FORMAT))
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) PutTaskSpecComment(c *db.TaskSpecComment) error {
	c.Timestamp = fixTimestamp(c.Timestamp)
	id := taskSpecCommentId(c)
	_, err := d.taskSpecComments().Doc(id).Create(context.Background(), c)
	if grpc.Code(err) == codes.AlreadyExists {
		return db.ErrAlreadyExists
	}
	return err
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) DeleteTaskSpecComment(c *db.TaskSpecComment) error {
	id := taskSpecCommentId(c)
	_, err := d.taskSpecComments().Doc(id).Delete(context.Background())
	return err
}

// commitCommentId returns an ID for the CommitComment.
func commitCommentId(c *db.CommitComment) string {
	// TODO(borenet): Use Firestore-assigned IDs.
	return fmt.Sprintf("%s#%s#%s", c.Repo, c.Revision, c.Timestamp.Format(local_db.TIMESTAMP_FORMAT))
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) PutCommitComment(c *db.CommitComment) error {
	c.Timestamp = fixTimestamp(c.Timestamp)
	id := commitCommentId(c)
	_, err := d.commitComments().Doc(id).Create(context.Background(), c)
	if grpc.Code(err) == codes.AlreadyExists {
		return db.ErrAlreadyExists
	}
	return err
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) DeleteCommitComment(c *db.CommitComment) error {
	id := commitCommentId(c)
	_, err := d.commitComments().Doc(id).Delete(context.Background())
	return err
}
