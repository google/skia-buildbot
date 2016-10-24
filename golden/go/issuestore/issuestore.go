package issuestore

import "github.com/boltdb/bolt"

type IssueStore interface {
	ByDigest(digest string) ([]string, error) // list of issues
	ByTrace(traceID string) ([]string, error) // list of issues
	ByTest(testName string) ([]string, error) // list of issues
	Get(issueID string) (*ConnectRec, error)
	Put(issueID string, rec *ConnectRec) error // create update
	Add(issueID string, rec *ConnectRec) error
	Remove(issueID string, rec *ConnectRec) error
	Delete(issueID string) error
}

type ConnectRec struct {
	IssueID   string   // id of the issue we want to annotate.
	Digests   []string // Example digests we are interested in.
	Traces    []string // Traces we are interested in.
	Ignores   []string // Ignores
	TestNames []string // TestNames
}

type boltIssueStore struct {
	db *bolt.DB
}

func New() (IssueStore, error) {
	db, err := bolt.Open("my.db", 0600, nil)
	if err != nil {
		return nil, err
	}
	return &boltIssueStore{
		db: db,
	}, nil
}
