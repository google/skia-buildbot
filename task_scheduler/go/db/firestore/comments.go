package firestore

import (
	"context"
	"fmt"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/task_scheduler/go/db"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	COLLECTION_COMMIT_COMMENTS    = "commit-comments"
	COLLECTION_TASK_COMMENTS      = "task-comments"
	COLLECTION_TASK_SPEC_COMMENTS = "task-spec-comments"
)

// commitComments returns a reference to the commit comments collection.
func (d *firestoreDB) commitComments() *fs.CollectionRef {
	return d.client.Collection(COLLECTION_COMMIT_COMMENTS)
}

// taskComments returns a reference to the task comments collection.
func (d *firestoreDB) taskComments() *fs.CollectionRef {
	return d.client.Collection(COLLECTION_TASK_COMMENTS)
}

// taskSpecComments returns a reference to the task spec comments collection.
func (d *firestoreDB) taskSpecComments() *fs.CollectionRef {
	return d.client.Collection(COLLECTION_TASK_SPEC_COMMENTS)
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

	ctx := context.Background()
	q := d.commitComments().Where("Timestamp", ">=", from).OrderBy("Timestamp", fs.Asc)
	if err := firestore.IterDocs(ctx, q, func() interface{} {
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

	q = d.taskComments().Where("Timestamp", ">=", from).OrderBy("Timestamp", fs.Asc)
	if err := firestore.IterDocs(ctx, q, func() interface{} {
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

	q = d.taskSpecComments().OrderBy("Timestamp", fs.Asc)
	if err := firestore.IterDocs(ctx, q, func() interface{} {
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
	return fmt.Sprintf("%s#%s#%s#%s", c.Repo, c.Revision, c.Name, fixTimestamp(c.Timestamp).Format(local_db.TIMESTAMP_FORMAT))
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) PutTaskComment(c *db.TaskComment) error {
	c.Timestamp = fixTimestamp(c.Timestamp)
	id := taskCommentId(c)
	ref := d.taskComments().Doc(id)
	return d.client.RunTransaction(context.Background(), func(ctx context.Context, tx *fs.Transaction) error {
		doc, err := tx.Get(ref)
		if grpc.Code(err) == codes.NotFound {
			return tx.Create(ref, c)
		} else if err != nil {
			return err
		}
		// The comment already exists. Assert that it's identical to
		// the passed-in comment.
		var old db.TaskComment
		if err := doc.DataTo(&old); err != nil {
			return err
		}
		if !deepequal.DeepEqual(&old, c) {
			return db.ErrAlreadyExists
		}
		return nil
	})
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) DeleteTaskComment(c *db.TaskComment) error {
	id := taskCommentId(c)
	_, err := d.taskComments().Doc(id).Delete(context.Background())
	return err
}

// taskSpecCommentId returns an ID for the TaskSpecComment.
func taskSpecCommentId(c *db.TaskSpecComment) string {
	return fmt.Sprintf("%s#%s#%s", c.Repo, c.Name, c.Timestamp.Format(local_db.TIMESTAMP_FORMAT))
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) PutTaskSpecComment(c *db.TaskSpecComment) error {
	c.Timestamp = fixTimestamp(c.Timestamp)
	id := taskSpecCommentId(c)
	ref := d.taskSpecComments().Doc(id)
	return d.client.RunTransaction(context.Background(), func(ctx context.Context, tx *fs.Transaction) error {
		doc, err := tx.Get(ref)
		if grpc.Code(err) == codes.NotFound {
			return tx.Create(ref, c)
		} else if err != nil {
			return err
		}
		// The comment already exists. Assert that it's identical to
		// the passed-in comment.
		var old db.TaskSpecComment
		if err := doc.DataTo(&old); err != nil {
			return err
		}
		if !deepequal.DeepEqual(&old, c) {
			return db.ErrAlreadyExists
		}
		return nil
	})
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) DeleteTaskSpecComment(c *db.TaskSpecComment) error {
	id := taskSpecCommentId(c)
	_, err := d.taskSpecComments().Doc(id).Delete(context.Background())
	return err
}

// commitCommentId returns an ID for the CommitComment.
func commitCommentId(c *db.CommitComment) string {
	return fmt.Sprintf("%s#%s#%s", c.Repo, c.Revision, c.Timestamp.Format(local_db.TIMESTAMP_FORMAT))
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) PutCommitComment(c *db.CommitComment) error {
	c.Timestamp = fixTimestamp(c.Timestamp)
	id := commitCommentId(c)
	ref := d.commitComments().Doc(id)
	return d.client.RunTransaction(context.Background(), func(ctx context.Context, tx *fs.Transaction) error {
		doc, err := tx.Get(ref)
		if grpc.Code(err) == codes.NotFound {
			return tx.Create(ref, c)
		} else if err != nil {
			return err
		}
		// The comment already exists. Assert that it's identical to
		// the passed-in comment.
		var old db.CommitComment
		if err := doc.DataTo(&old); err != nil {
			return err
		}
		if !deepequal.DeepEqual(&old, c) {
			return db.ErrAlreadyExists
		}
		return nil
	})
}

// See documentation for db.CommentDB interface.
func (d *firestoreDB) DeleteCommitComment(c *db.CommitComment) error {
	id := commitCommentId(c)
	_, err := d.commitComments().Doc(id).Delete(context.Background())
	return err
}
