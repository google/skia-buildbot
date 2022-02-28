package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	fs "cloud.google.com/go/firestore"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	// Collection name for Configs.
	collectionConfigs = "Configs"

	// Firestore-related constants.
	defaultAttempts = 3
	defaultTimeout  = 10 * time.Second
)

var (
	ErrNotFound = errors.New("Request with given ID does not exist.")
)

// DB provides methods for interacting with a database of Configs.
type DB interface {
	// Close cleans up resources associated with the DB.
	Close() error

	// Get returns the Config for the given roller.
	Get(ctx context.Context, rollerID string) (*config.Config, error)

	// GetAll returns Configs for all known rollers.
	GetAll(ctx context.Context) ([]*config.Config, error)

	// Put inserts the Config into the DB. Implementations MUST validate the
	// Config before inserting into the DB.
	Put(ctx context.Context, rollerID string, cfg *config.Config) error

	// Delete removes the Config for the given roller from the DB.
	Delete(ctx context.Context, rollerID string) error
}

// firestoreB is a DB implementation backed by Firestore.
type FirestoreDB struct {
	client *firestore.Client
	coll   *fs.CollectionRef
}

// NewDB returns a DB instance backed by the given firestore.Client.
func NewDB(ctx context.Context, client *firestore.Client) (*FirestoreDB, error) {
	db := &FirestoreDB{
		client: client,
		coll:   client.Collection(collectionConfigs),
	}
	return db, nil
}

// NewDBWithParams returns a DB instance backed by Firestore, using the given
// params.
func NewDBWithParams(ctx context.Context, project, namespace, instance string, ts oauth2.TokenSource) (*FirestoreDB, error) {
	client, err := firestore.NewClient(ctx, project, namespace, instance, ts)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return NewDB(ctx, client)
}

// Close implements DB.
func (d *FirestoreDB) Close() error {
	return d.client.Close()
}

// Get implements DB.
func (d *FirestoreDB) Get(ctx context.Context, rollerID string) (*config.Config, error) {
	ref := d.coll.Doc(rollerID)
	doc, err := d.client.Get(ctx, ref, defaultAttempts, defaultTimeout)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		} else {
			return nil, skerr.Wrap(err)
		}
	}
	return decodeConfig(doc.Data())
}

// GetAll implements DB.
func (d *FirestoreDB) GetAll(ctx context.Context) ([]*config.Config, error) {
	rv := []*config.Config{}
	if err := d.client.IterDocs(ctx, "GetAll", "GetAll", d.coll.Query, defaultAttempts, defaultTimeout, func(doc *fs.DocumentSnapshot) error {
		cfg, err := decodeConfig(doc.Data())
		if err != nil {
			return skerr.Wrap(err)
		}
		rv = append(rv, cfg)
		return nil
	}); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// Put implements DB.
func (d *FirestoreDB) Put(ctx context.Context, rollerID string, cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := encodeConfig(cfg)
	if err != nil {
		return skerr.Wrap(err)
	}
	ref := d.coll.Doc(rollerID)
	if _, err := ref.Set(ctx, data); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// Delete implements DB.
func (d *FirestoreDB) Delete(ctx context.Context, rollerID string) error {
	ref := d.coll.Doc(rollerID)
	if _, err := ref.Delete(ctx); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// encodeConfig converts the config.Config to a map[string]interface which is
// able to be stored in Firestore.
func encodeConfig(cfg *config.Config) (map[string]interface{}, error) {
	b, err := protojson.Marshal(cfg)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	var rv map[string]interface{}
	if err := json.Unmarshal(b, &rv); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// decodeConfig converts the map[string]interface retrieved from Firestore to a
// config.Config.
func decodeConfig(data map[string]interface{}) (*config.Config, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	cfg := new(config.Config)
	if err := protojson.Unmarshal(b, cfg); err != nil {
		return nil, skerr.Wrap(err)
	}
	return cfg, nil
}
