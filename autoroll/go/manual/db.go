package manual

import (
	"context"
	"errors"
	"sync"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
)

const (
	// Collection name for ManualRollRequests.
	COLLECTION_REQUESTS = "ManualRollRequests"

	// App name used for Firestore.
	FS_APP = "autoroll-manual-rolls"

	// Possible values for ManualRollResult.
	RESULT_UNKNOWN = ""
	RESULT_FAILURE = "FAILURE"
	RESULT_SUCCESS = "SUCCESS"

	// Possible values for ManualRollStatus.
	STATUS_PENDING  = "PENDING"
	STATUS_STARTED  = "STARTED"
	STATUS_COMPLETE = "COMPLETED"
)

var (
	// All valid results.
	VALID_RESULTS = []string{
		RESULT_UNKNOWN,
		RESULT_FAILURE,
		RESULT_SUCCESS,
	}

	// All valid statuses.
	VALID_STATUSES = []string{
		STATUS_PENDING,
		STATUS_STARTED,
		STATUS_COMPLETE,
	}
)

// ManualRollResult represents the result of a manual roll.
type ManualRollResult string

// ManualRollStatus represents the status of a manual roll.
type ManualRollStatus string

// ManualRollRequest represents a request for a manual roll.
type ManualRollRequest struct {
	Requester  string           `json:"requester"`
	Result     ManualRollResult `json:"result,omitempty"`
	Revision   string           `json:"revision"`
	RollerName string           `json:"rollerName"`
	Status     ManualRollStatus `json:"status"`
	Timestamp  time.Time        `json:"timestamp"`
	Url        string           `json:"url,omitempty"`
}

// Validate the ManualRollRequest.
func (r *ManualRollRequest) Validate() error {
	if r.Requester == "" {
		return errors.New("Requester is required.")
	} else if !util.In(string(r.Result), VALID_RESULTS) {
		return errors.New("Invalid result.")
	} else if r.Revision == "" {
		return errors.New("Revision is required.")
	} else if r.RollerName == "" {
		return errors.New("RollerName is required.")
	} else if !util.In(string(r.Status), VALID_STATUSES) {
		return errors.New("Invalid status.")
	} else if util.TimeIsZero(r.Timestamp) {
		return errors.New("Timestamp is required.")
	}
	if r.Url == "" && r.Status != STATUS_PENDING {
		return errors.New("Url is required for non-pending requests.")
	}
	return nil
}

// DB provides methods for interacting with a database of ManualRollRequests.
type DB interface {
	// Close cleans up resources associated with the DB.
	Close() error

	// Return recent ManualRollRequests for the given roller, up to the given limit.
	GetRecent(rollerName string, limit int) ([]*ManualRollRequest, error)

	// Return all incomplete ManualRollRequests for the given roller.
	GetIncomplete(rollerName string) ([]*ManualRollRequest, error)

	// Insert the given ManualRollRequest. The DB implementation is
	// responsible for calling req.Validate().
	Insert(req *ManualRollRequest) error
}

// firestoreDB is a DB implementation backed by Firestore.
type firestoreDB struct {
	client *firestore.Client
	coll   *fs.CollectionRef
}

// NewDB returns a DB instance backed by Firestore.
func NewDB(ctx context.Context, project, instance string, ts oauth2.TokenSource) (DB, error) {
	client, err := firestore.NewClient(ctx, project, FS_APP, instance, ts)
	if err != nil {
		return nil, err
	}
	db := &firestoreDB{
		client: client,
		coll:   client.Collection(COLLECTION_REQUESTS),
	}
	return db, nil
}

// See documentation for DB interface.
func (d *firestoreDB) Close() error {
	return d.client.Close()
}

// See documentation for DB interface.
func (d *firestoreDB) GetRecent(rollerName string, limit int) ([]*ManualRollRequest, error) {
	/*	rv := []*ManualRollRequest{}
		q := b.coll.Query.Where(KEY_ROLLER_NAME, "==", rollerName)
	*/
	return nil, nil
}

// See documentation for DB interface.
func (d *firestoreDB) GetIncomplete(rollerName string) ([]*ManualRollRequest, error) {
	return nil, nil
}

// See documentation for DB interface.
func (d *firestoreDB) Insert(req *ManualRollRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	return nil
}

// memoryDB is a simple, in-memory DB implementation.
type memoryDB struct {
	data map[string][]*ManualRollRequest
	mtx  sync.RWMutex
}

// NewInMemoryDB returns an in-memory DB instance.
func NewInMemoryDB() DB {
	return &memoryDB{
		data: map[string][]*ManualRollRequest{},
	}
}

// See documentation for DB interface.
func (d *memoryDB) Close() error {
	return nil
}

// See documentation for DB interface.
func (d *memoryDB) GetRecent(rollerName string, limit int) ([]*ManualRollRequest, error) {
	d.mtx.RLock()
	defer d.mtx.RUnlock()
	all := d.data[rollerName]
	rv := []*ManualRollRequest{}
	for i := len(all) - 1; i >= 0 && i >= len(all)-limit; i-- {
		rv = append(rv, all[i])
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *memoryDB) GetIncomplete(rollerName string) ([]*ManualRollRequest, error) {
	d.mtx.RLock()
	defer d.mtx.RUnlock()
	rv := []*ManualRollRequest{}
	for _, r := range d.data[rollerName] {
		if r.Status != STATUS_COMPLETE {
			rv = append(rv, r)
		}
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *memoryDB) Insert(req *ManualRollRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	d.mtx.Lock()
	defer d.mtx.Unlock()
	d.data[req.RollerName] = append(d.data[req.RollerName], req)
	return nil
}
