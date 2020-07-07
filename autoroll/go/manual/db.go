package manual

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	fs "cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// Collection name for ManualRollRequests.
	COLLECTION_REQUESTS = "ManualRollRequests"

	// App name used for Firestore.
	FS_APP = "autoroll"

	// Possible values for ManualRollResult.
	RESULT_UNKNOWN = ManualRollResult("")
	RESULT_FAILURE = ManualRollResult("FAILURE")
	RESULT_SUCCESS = ManualRollResult("SUCCESS")

	// Possible values for ManualRollStatus.
	STATUS_PENDING  = ManualRollStatus("PENDING")
	STATUS_STARTED  = ManualRollStatus("STARTED")
	STATUS_COMPLETE = ManualRollStatus("COMPLETED")

	// Firestore-related constants.
	KEY_ROLLER_NAME = "RollerName"
	KEY_STATUS      = "Status"
	KEY_TIMESTAMP   = "Timestamp"

	DEFAULT_ATTEMPTS = 3

	QUERY_TIMEOUT  = 60 * time.Second
	INSERT_TIMEOUT = 10 * time.Second
)

var (
	// All valid results.
	VALID_RESULTS = []ManualRollResult{
		RESULT_UNKNOWN,
		RESULT_FAILURE,
		RESULT_SUCCESS,
	}

	// All valid statuses.
	VALID_STATUSES = []ManualRollStatus{
		STATUS_PENDING,
		STATUS_STARTED,
		STATUS_COMPLETE,
	}

	ErrConcurrentUpdate = errors.New("Concurrent update.")
	ErrNotFound         = errors.New("Request with given ID does not exist.")
)

// ManualRollResult represents the result of a manual roll.
type ManualRollResult string

// Validate the ManualRollResult.
func (r ManualRollResult) Validate() error {
	for _, v := range VALID_RESULTS {
		if v == r {
			return nil
		}
	}
	return errors.New("Invalid result.")
}

// ManualRollStatus represents the status of a manual roll.
type ManualRollStatus string

// Validate the ManualRollStatus.
func (r ManualRollStatus) Validate() error {
	for _, v := range VALID_STATUSES {
		if v == r {
			return nil
		}
	}
	return errors.New("Invalid status.")
}

// ManualRollRequest represents a request for a manual roll.
type ManualRollRequest struct {
	Id            string           `json:"id"`
	DbModified    time.Time        `json:"-"`
	Requester     string           `json:"requester"`
	Result        ManualRollResult `json:"result,omitempty"`
	ResultDetails string           `json:"resultDetails,omitempty"`
	Revision      string           `json:"revision"`
	RollerName    string           `json:"rollerName"`
	Status        ManualRollStatus `json:"status"`
	Timestamp     time.Time        `json:"timestamp"`
	Url           string           `json:"url,omitempty"`

	DryRun bool `json:"dry_run"`
	// If Emails is empty then the requester and sheriffs will be emailed.
	Emails []string `json:"emails"`
	// Do not call rm.GetRevision(Revision) if this is true. Use Revision{Id: Revision} instead.
	NoResolveRevision bool `json:"no_resolve_revision"`
}

// Return a copy of the ManualRollRequest.
func (r *ManualRollRequest) Copy() *ManualRollRequest {
	return &ManualRollRequest{
		Id:            r.Id,
		DbModified:    r.DbModified,
		Requester:     r.Requester,
		Result:        r.Result,
		ResultDetails: r.ResultDetails,
		Revision:      r.Revision,
		RollerName:    r.RollerName,
		Status:        r.Status,
		Timestamp:     r.Timestamp,
		Url:           r.Url,

		DryRun:            r.DryRun,
		Emails:            util.CopyStringSlice(r.Emails),
		NoResolveRevision: r.NoResolveRevision,
	}
}

// Validate the ManualRollRequest.
func (r *ManualRollRequest) Validate() error {
	if r.Requester == "" {
		return errors.New("Requester is required.")
	} else if err := r.Result.Validate(); err != nil {
		return err
	} else if r.Revision == "" {
		return errors.New("Revision is required.")
	} else if r.RollerName == "" {
		return errors.New("RollerName is required.")
	} else if err := r.Status.Validate(); err != nil {
		return err
	} else if util.TimeIsZero(r.Timestamp) {
		return errors.New("Timestamp is required.")
	}
	if r.Timestamp != firestore.FixTimestamp(r.Timestamp) {
		return errors.New("Timestamp must be in UTC and truncated to microsecond precision.")
	}
	if r.DbModified != firestore.FixTimestamp(r.DbModified) {
		return errors.New("DbModified must be in UTC and truncated to microsecond precision.")
	}
	if r.Status == STATUS_PENDING {
		if r.Url != "" {
			return errors.New("Url is invalid for pending requests.")
		}
		if r.Result != RESULT_UNKNOWN {
			return errors.New("Result is invalid for pending requests.")
		}
	} else {
		if r.Url == "" && r.Result != RESULT_FAILURE {
			return errors.New("Url is required for non-pending, non-failed requests.")
		}
		if r.Status == STATUS_STARTED && r.Result != RESULT_UNKNOWN {
			return errors.New("Result is invalid for running requests.")
		} else if r.Status != STATUS_STARTED && r.Result == RESULT_UNKNOWN {
			return errors.New("Result is required for finished requests.")
		}
	}
	if r.Id == "" && !util.TimeIsZero(r.DbModified) {
		return errors.New("Request has no ID but has non-zero DbModified timestamp.")
	} else if r.Id != "" && util.TimeIsZero(r.DbModified) {
		return errors.New("Request has an ID but has a zero DbModified timestamp.")
	}
	return nil
}

// DB provides methods for interacting with a database of ManualRollRequests.
type DB interface {
	// Close cleans up resources associated with the DB.
	Close() error

	// Return recent ManualRollRequests for the given roller, up to the
	// given limit, in reverse chronological order.
	GetRecent(rollerName string, limit int) ([]*ManualRollRequest, error)

	// Return all incomplete ManualRollRequests for the given roller.
	GetIncomplete(rollerName string) ([]*ManualRollRequest, error)

	// Insert the given ManualRollRequest. The DB implementation is
	// responsible for calling req.Validate(). If the request has an ID,
	// the existing entry in the DB is updated. Otherwise, a new entry is
	// inserted. If the entry already exist, the version in the DB must have
	// the same DbModified timestamp, or ErrConcurrentUpdate is returned.
	Put(req *ManualRollRequest) error
}

// firestoreDB is a DB implementation backed by Firestore.
type firestoreDB struct {
	client *firestore.Client
	coll   *fs.CollectionRef
}

// NewDB returns a DB instance backed by the given firestore.Client.
func NewDB(ctx context.Context, client *firestore.Client) (DB, error) {
	db := &firestoreDB{
		client: client,
		coll:   client.Collection(COLLECTION_REQUESTS),
	}
	return db, nil
}

// NewDB returns a DB instance backed by Firestore, using the given params.
func NewDBWithParams(ctx context.Context, project, instance string, ts oauth2.TokenSource) (DB, error) {
	client, err := firestore.NewClient(ctx, project, FS_APP, instance, ts)
	if err != nil {
		return nil, err
	}
	return NewDB(ctx, client)
}

// See documentation for DB interface.
func (d *firestoreDB) Close() error {
	return d.client.Close()
}

// See documentation for DB interface.
func (d *firestoreDB) GetRecent(rollerName string, limit int) ([]*ManualRollRequest, error) {
	rv := []*ManualRollRequest{}
	q := d.coll.Query.Where(KEY_ROLLER_NAME, "==", rollerName).OrderBy(KEY_TIMESTAMP, fs.Desc).Limit(limit)
	if err := d.client.IterDocs(context.TODO(), "GetRecent", fmt.Sprintf("%s-%d", rollerName, limit), q, DEFAULT_ATTEMPTS, QUERY_TIMEOUT, func(doc *fs.DocumentSnapshot) error {
		var req ManualRollRequest
		if err := doc.DataTo(&req); err != nil {
			return err
		}
		rv = append(rv, &req)
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *firestoreDB) GetIncomplete(rollerName string) ([]*ManualRollRequest, error) {
	rv := []*ManualRollRequest{}
	// Firestore doesn't support a "!=" operator for Query.Where(), so we
	// have to use multiple requests.
	for _, status := range VALID_STATUSES {
		if status == STATUS_COMPLETE {
			continue
		}
		q := d.coll.Query.Where(KEY_ROLLER_NAME, "==", rollerName).Where(KEY_STATUS, "==", status).OrderBy(KEY_TIMESTAMP, fs.Desc)
		if err := d.client.IterDocs(context.TODO(), "GetIncomplete", rollerName, q, DEFAULT_ATTEMPTS, QUERY_TIMEOUT, func(doc *fs.DocumentSnapshot) error {
			var req ManualRollRequest
			if err := doc.DataTo(&req); err != nil {
				return err
			}
			rv = append(rv, &req)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *firestoreDB) Put(req *ManualRollRequest) (rvErr error) {
	if err := req.Validate(); err != nil {
		return err
	}
	isNew := req.Id == ""
	if isNew {
		req.Id = firestore.AlphaNumID()
	}
	oldDbModified := req.DbModified
	req.DbModified = firestore.FixTimestamp(time.Now())
	if oldDbModified.Equal(req.DbModified) {
		// Prevent inserting the request with unchanged DbModified timestamp.
		req.DbModified = oldDbModified.Add(firestore.TS_RESOLUTION)
	}
	defer func() {
		if rvErr != nil {
			if isNew {
				req.Id = ""
			}
			req.DbModified = oldDbModified
		}
	}()
	return d.client.RunTransaction(context.TODO(), "Put", req.Id, DEFAULT_ATTEMPTS, INSERT_TIMEOUT, func(ctx context.Context, tx *fs.Transaction) error {
		ref := d.coll.Doc(req.Id)
		doc, err := tx.Get(ref)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				if !isNew {
					return ErrNotFound
				}
			} else {
				return err
			}
		}
		if err == nil && isNew {
			sklog.Errorf("Request is new but previous version was found in the DB: %s", req.Id)
			return ErrConcurrentUpdate
		}
		if !isNew {
			var old ManualRollRequest
			if err := doc.DataTo(&old); err != nil {
				return err
			}
			if !old.DbModified.Equal(oldDbModified) {
				return ErrConcurrentUpdate
			}
		}
		return tx.Set(ref, req)
	})
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
		rv = append(rv, all[i].Copy())
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
			rv = append(rv, r.Copy())
		}
	}
	return rv, nil
}

// See documentation for DB interface.
func (d *memoryDB) Put(req *ManualRollRequest) (rvErr error) {
	if err := req.Validate(); err != nil {
		return err
	}
	isNew := req.Id == ""
	d.mtx.Lock()
	defer d.mtx.Unlock()
	oldDbModified := req.DbModified
	req.DbModified = time.Now().UTC().Truncate(firestore.TS_RESOLUTION)
	defer func() {
		if rvErr != nil {
			req.DbModified = oldDbModified
		}
	}()
	if isNew {
		req.Id = uuid.New().String()
		d.data[req.RollerName] = append(d.data[req.RollerName], req.Copy())
		return nil
	}
	for idx, r := range d.data[req.RollerName] {
		if r.Id == req.Id {
			if !r.DbModified.Equal(oldDbModified) {
				return ErrConcurrentUpdate
			}
			d.data[req.RollerName][idx] = req.Copy()
			return nil
		}
	}
	return ErrNotFound
}
