// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tumble

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/appengine/meta"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
	"golang.org/x/net/context"
)

var registry = map[string]reflect.Type{}

// Register allows |mut| to be played by the tumble backend. This should be
// called at init() time once for every Mutation implementation.
//
// This will also gob.Register your mutation implementation.
//
// Example:
//   Register((*MyMutationImpl)(nil))
func Register(mut Mutation) {
	gob.Register(mut)
	t := reflect.TypeOf(mut)
	registry[t.String()] = t
}

// Mutation is the interface that your tumble mutations must implement.
//
// Mutation implementations can be registered with the Register function.
type Mutation interface {
	// Root returns a datastore.Key which will be used to derive the Key for the
	// entity group which this Mutation will operate on. This is used to batch
	// together Entries for more efficient processing.
	//
	// Returning nil is an error.
	Root(c context.Context) *datastore.Key

	// RollForward performs the action of the Mutation.
	//
	// It is only considered successful if it returns nil. If it returns non-nil,
	// then it will be retried at a later time. If it never returns nil, then it
	// will never be flushed from tumble's queue, and you'll have to manually
	// delete it or fix the code so that it can be handled without error.
	//
	// This method runs inside of a single-group transaction. It must modify only
	// the entity group specified by Root().
	//
	// As a side effect, RollForward may return new arbitrary Mutations. These
	// will be committed in the same transaction as RollForward.
	//
	// The context contains an implementation of "luci/gae/service/datastore",
	// using the "luci/gae/filter/txnBuf" transaction buffer. This means that
	// all functionality (including additional transactions) is available, with
	// the limitations mentioned by that package (notably, no cursors are
	// allowed).
	RollForward(c context.Context) ([]Mutation, error)
}

// DelayedMutation is a Mutation which allows you to defer its processing
// until a certain absolute time.
//
// As a side effect, tumble will /mostly/ process items in their chronological
// ProcessAfter order, instead of the undefined order.
//
// Your tumble configuration must have DelayedMutations set, and you must have
// added the appropriate index to use these. If DelayedMutations is not set,
// then tumble will ignore the ProcessAfter and HighPriorty values here, and
// process mutations as quickly as possible in no particular order.
type DelayedMutation interface {
	Mutation

	// ProcessAfter will be called once when scheduling this Mutation. The
	// mutation will be recorded to datastore immediately, but tumble will skip it
	// for processing until at least the time that's returned here. Multiple calls
	// to this method should always return the same time.
	//
	// A Time value in the past will get reset to "next available time slot",
	// unless HighPriority() returns true.
	ProcessAfter() time.Time

	// HighPriority indicates that this mutation should be processed before
	// others, if possible, and must be set in conjunction with a ProcessAfter
	// timestamp that occurs in the past.
	//
	// Tumble works by processing Mutations in the order of their creation, or
	// ProcessAfter times, whichever is later. If HighPriority is true, then a
	// ProcessAfter time in the past will take precedence over Mutations which
	// may actually have been recorded after this one, in the event that tumble
	// is processing tasks slower than they're being created.
	HighPriority() bool
}

type realMutation struct {
	// TODO(riannucci): add functionality to luci/gae/service/datastore so that
	// GetMeta/SetMeta may be overridden by the struct.
	_kind  string         `gae:"$kind,tumble.Mutation"`
	ID     string         `gae:"$id"`
	Parent *datastore.Key `gae:"$parent"`

	ExpandedShard int64
	ProcessAfter  time.Time
	TargetRoot    *datastore.Key

	Version string
	Type    string
	Data    []byte `gae:",noindex"`
}

func (r *realMutation) shard(cfg *Config) taskShard {
	expandedShardsPerShard := math.MaxUint64 / cfg.NumShards
	ret := uint64(r.ExpandedShard-math.MinInt64) / expandedShardsPerShard
	// account for rounding errors on the last shard.
	if ret >= cfg.NumShards {
		ret = cfg.NumShards - 1
	}
	return taskShard{ret, mkTimestamp(cfg, r.ProcessAfter)}
}

func putMutations(c context.Context, cfg *Config, fromRoot *datastore.Key, muts []Mutation, round uint64) (
	shardSet map[taskShard]struct{}, mutKeys []*datastore.Key, err error) {
	if len(muts) == 0 {
		return
	}

	version, err := meta.GetEntityGroupVersion(c, fromRoot)
	if err != nil {
		return
	}

	ds := datastore.Get(c)

	now := clock.Now(c).UTC()

	shardSet = map[taskShard]struct{}{}
	toPut := make([]*realMutation, len(muts))
	mutKeys = make([]*datastore.Key, len(muts))
	for i, m := range muts {
		id := fmt.Sprintf("%016x_%08x_%08x", version, round, i)
		toPut[i], err = newRealMutation(c, cfg, id, fromRoot, m, now)
		if err != nil {
			logging.Errorf(c, "error creating real mutation for %v: %s", m, err)
			return
		}
		mutKeys[i] = ds.KeyForObj(toPut[i])

		shardSet[toPut[i].shard(cfg)] = struct{}{}
	}

	if err = ds.Put(toPut); err != nil {
		logging.Errorf(c, "error putting %d new mutations: %s", len(toPut), err)
	}
	return
}

var appVersion = struct {
	sync.Once
	version string
}{}

func getAppVersion(c context.Context) string {
	appVersion.Do(func() {
		appVersion.version = info.Get(c).VersionID()

		// AppEngine version is <app-yaml-version>.<unique-upload-id>
		//
		// The upload ID prevents version consistency between different AppEngine
		// modules, which will necessarily have different IDs, so we base our
		// comparable version off of the app.yaml-supplied value.
		if idx := strings.LastIndex(appVersion.version, "."); idx > 0 {
			appVersion.version = appVersion.version[:idx]
		}
	})
	return appVersion.version
}

func newRealMutation(c context.Context, cfg *Config, id string, parent *datastore.Key, m Mutation, now time.Time) (*realMutation, error) {
	when := now
	if cfg.DelayedMutations {
		if dm, ok := m.(DelayedMutation); ok {
			targetTime := dm.ProcessAfter()
			if dm.HighPriority() || targetTime.After(now) {
				when = targetTime
			}
		}
	}

	t := reflect.TypeOf(m).String()
	if _, ok := registry[t]; !ok {
		return nil, fmt.Errorf("Attempting to add unregistered mutation %v: %v", t, m)
	}

	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(m)
	if err != nil {
		return nil, err
	}

	root := m.Root(c).Root()

	hash := sha1.Sum([]byte(root.Encode()))
	eshard := int64(binary.BigEndian.Uint64(hash[:]))

	return &realMutation{
		ID:     id,
		Parent: parent,

		ExpandedShard: eshard,
		ProcessAfter:  when,
		TargetRoot:    root,

		Version: getAppVersion(c),
		Type:    t,
		Data:    buf.Bytes(),
	}, nil
}

func (r *realMutation) GetMutation() (Mutation, error) {
	typ, ok := registry[r.Type]
	if !ok {
		return nil, fmt.Errorf("unable to load reflect.Type for %q", r.Type)
	}

	ret := reflect.New(typ)
	if err := gob.NewDecoder(bytes.NewBuffer(r.Data)).DecodeValue(ret); err != nil {
		return nil, err
	}

	return ret.Elem().Interface().(Mutation), nil
}
