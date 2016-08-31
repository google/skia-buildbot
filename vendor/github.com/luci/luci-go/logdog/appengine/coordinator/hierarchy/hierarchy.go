// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package hierarchy

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	"github.com/luci/luci-go/logdog/common/types"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

// componentEntity is a hierarchial component that stores a specific log path
// component. A given stream path is broken into components, each of which is
// represented by a single componentEntity.
//
// A componentEntity is keyed based on its path component name and the hash of
// its parent component's path. This ensures that the set of child components
// can be obtained for any parent component path.
//
// The child component's key is chosen to be sortable with the following
// preferences:
//	- Stream components sort before path components.
//	- Stream components sort alphanumerically. Elements that are entirely
//	  numeric will be constructed to sort numerically.
//
// Storing the component's name in its key ensures that a keys query will pull
// the set of elements in the correct order, requiring no non-default indexes.
//
// Writing path components is inherently idempotent, since each one is defined
// solely by its identity. Consequently, transactions are not necessary when
// writing path components.
//
// All componentEntity share a common implicit ancestor, "/".
//
// This entity is created at stream and prefix registration, and is entirely
// centered around being queried for log stream "directory" listings. For
// example:
//
//	foo/bar/+/baz/qux
//
// This stream would add the following name components to the datastore, keyed
// (parent-key, id) as:
//	("/", "foo")
//	("/foo", "bar")
//	("/foo/bar", "+")
//	("/foo/bar/+", "baz")
//	("/foo/bar/+/baz", "qux")
//
// A generated property, "s", is stored to indicate whether this entry is a
// stream or path component.
type componentEntity struct {
	// _kind is the entity's kind. This is intentionally short to mitigate index
	// bloat since this will be repeated in a lot of keys.
	_kind string `gae:"$kind,_StreamNameComponent"`

	// ID is the name component of this specific stream.
	ID componentID `gae:"$id"`
}

var _ ds.PropertyLoadSaver = (*componentEntity)(nil)

func (c *componentEntity) Load(pmap ds.PropertyMap) error {
	// Delete custom elements (added in Save).
	delete(pmap, "s")
	delete(pmap, "p")

	return ds.GetPLS(c).Load(pmap)
}

func (c *componentEntity) Save(withMeta bool) (ds.PropertyMap, error) {
	pmap, err := ds.GetPLS(c).Save(withMeta)
	if err != nil {
		return nil, err
	}
	pmap["s"] = []ds.Property{ds.MkProperty(c.ID.stream)}
	pmap["p"] = []ds.Property{ds.MkProperty(c.ID.parent)}
	return pmap, nil
}

// streamPath returns the StreamPath of this component, given its parent path.
//
// This is only valid if the component is a stream component.
func (c *componentEntity) streamPath(parent types.StreamPath) types.StreamPath {
	return parent.Append(c.ID.name)
}

func componentEntityParent(parent types.StreamPath) string {
	hash := sha256.Sum256([]byte(parent))
	return hex.EncodeToString(hash[:])
}

func mkComponentEntity(parent types.StreamPath, name string, stream bool) *componentEntity {
	return &componentEntity{
		ID: componentID{
			parent: componentEntityParent(parent),
			name:   name,
			stream: stream,
		},
	}
}

// Request is a listing request to execute.
//
// It describes a hierarchy listing request. For example, given the following
// streams in project "qux":
//	qux/foo/+/bar
//	qux/foo/+/bar/baz
//	qux/foo/bar/+/baz
//
// The following queries would return values ("$" denotes streams vs. paths):
//	Project="", Path="": ["qux"]
//	Project="qux", Path="": ["foo"]
//	Project="qux", Path="foo": ["+", "bar"]
//	Project="qux", Path="foo/+": ["bar$", "bar"]
//	Project="qux", Path="foo/bar": ["+"]
//	Project="qux", Path="foo/bar/+": ["baz$"]
//
// If Limit is >0, it will be used to constrain the results. Otherwise, all
// results will be returned.
//
// If Next is not empty, it is a datastore cursor for a continued query. If
// supplied, it must use the same parameters as the previous queries in the
// sequence.
type Request struct {
	// Project is the project to list. If empty, Request will perform a
	// project-level listing.
	Project string
	// PathBase is the base path within Project to list.
	PathBase string
	// StreamOnly, if true, only returns stream path components.
	StreamOnly bool

	// Limit, if >0, is the maximum number of results to return. If more results
	// are available, the returned List will have its Next field set to a cursor
	// that can be used to issue iterative requests.
	Limit int
	// Next, if not empty, is the start cursor for this stream.
	Next string
	// Skip, if >0, skips past the first Skip query results.
	Skip int
}

// ListComponent is a single component element in the stream path hierarchy.
//
// This can represent a path component "path" in (path/component/+/stream)
// or a stream component ("stream").
type ListComponent struct {
	// Name is the name of this hierarchy element.
	Name string
	// Stream, if true, indicates that this is a stream component.
	Stream bool
}

// List is a branch of the stream path tree.
//
// It may represent either the top-level project hierarchy, or the sub-project
// stream space hierarchy, depending on the query base.
type List struct {
	// Project is the listed project name. If empty, the list refers to the
	// project namespace.
	Project config.ProjectName
	// PathBase is the stream path base.
	PathBase types.StreamPath
	// Comp is the set of elements in this hierarchy result.
	Comp []*ListComponent

	// Next, if not empty, is the iterative query cursor.
	Next string
}

// Path returns the StreamPath of the supplied Component.
func (l *List) Path(c *ListComponent) types.StreamPath {
	return l.PathBase.Append(c.Name)
}

// Get performs a hierarchy query based on parameters in the supplied Request
// and returns the resulting List.
//
// This method will set the namespace based on the request after asserting user
// membership.
//
// The supplied Context should not be bound to a namespace (i.e., default
// namespace).
//
// If a failure is encountered, a wrapped gRPC error will be returned.
func Get(c context.Context, r Request) (*List, error) {
	// If our project is empty, this is a project-level query.
	if r.Project == "" {
		return getProjects(c, &r)
	}

	// Build our List result.
	//
	// We will validate our Project and PathBase types immediately afterwards.
	l := List{
		Project:  config.ProjectName(r.Project),
		PathBase: types.StreamPath(r.PathBase),
	}

	// Validate our PathBase component.
	if err := l.PathBase.ValidatePartial(); err != nil {
		return nil, grpcutil.Errf(codes.InvalidArgument, "invalid stream path base %q: %v", l.PathBase, err)
	}

	// Enter the supplied Project namespace. This will assert the the user has
	// access to the project.
	if err := coordinator.WithProjectNamespace(&c, l.Project, coordinator.NamespaceAccessREAD); err != nil {
		return nil, err
	}

	// Determine our ancestor component.
	di := ds.Get(c)

	q := ds.NewQuery("_StreamNameComponent")
	q = q.Eq("p", componentEntityParent(l.PathBase))
	if r.StreamOnly {
		q = q.Eq("s", true)
	}
	if r.Next != "" {
		k, err := keyForCursor(di, r.Next)
		if err != nil {
			return nil, grpcutil.Errf(codes.InvalidArgument, "invalid cursor: %s", err)
		}
		q = q.Gt("__key__", k)
	}
	if r.Skip > 0 {
		q = q.Offset(int32(r.Skip))
	}

	limit := r.Limit
	if limit > 0 {
		q = q.Limit(int32(limit))
	}

	err := di.Run(q, func(e *componentEntity) error {
		l.Comp = append(l.Comp, &ListComponent{
			Name:   e.ID.name,
			Stream: e.ID.stream,
		})

		if limit > 0 && len(l.Comp) >= limit {
			l.Next = cursorForKey(di, e)
			return ds.Stop
		}
		return nil
	})
	if err != nil {
		log.WithError(err).Errorf(c, "Failed to execute hierarhcy query.")
		return nil, grpcutil.Internal
	}
	return &l, nil
}

// keyForCursor returns the component key for the supplied cursor.
//
// If the cursor string is not valid, an error will be returned.
func keyForCursor(di ds.Interface, c string) (*ds.Key, error) {
	d, err := base64.URLEncoding.DecodeString(c)
	if err != nil {
		return nil, err
	}

	return di.NewKey("_StreamNameComponent", string(d), 0, nil), nil
}

// cursorForKey returns a cursor for the supplied componentID. This cursor will
// start new queries at the component immediately following this ID.
func cursorForKey(di ds.Interface, e *componentEntity) string {
	key := di.KeyForObj(e)
	return base64.URLEncoding.EncodeToString([]byte(key.StringID()))
}
