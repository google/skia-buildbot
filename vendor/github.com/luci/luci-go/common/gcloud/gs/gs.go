// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gs

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/retry"

	gs "cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

var (
	// ReadWriteScopes is the set of scopes needed for read/write Google Storage
	// access.
	ReadWriteScopes = []string{gs.ScopeReadWrite}

	// ReadOnlyScopes is the set of scopes needed for read/write Google Storage
	// read-only access.
	ReadOnlyScopes = []string{gs.ScopeReadOnly}
)

// Client abstracts funcitonality to connect with and use Google Storage from
// the actual Google Storage client.
//
// Non-production implementations are used primarily for testing.
type Client interface {
	io.Closer

	// NewReader instantiates a new Reader instance for the named bucket/path.
	//
	// The supplied offset must be >= 0, or else this function will panic.
	//
	// If the supplied length is <0, no upper byte bound will be set.
	NewReader(p Path, offset, length int64) (io.ReadCloser, error)

	// NewWriter instantiates a new Writer instance for the named bucket/path.
	NewWriter(p Path) (Writer, error)

	// Delete deletes the object at the specified path.
	//
	// If the object does not exist, it is considered a success.
	Delete(p Path) error

	// Rename renames an object from one path to another.
	//
	// NOTE: The object should be removed from its original path, but current
	// implementation uses two operations (Copy + Delete), so it may
	// occasionally fail.
	Rename(src, dst Path) error
}

// prodGSObject is an implementation of Client interface using the production
// Google Storage client.
type prodClient struct {
	context.Context

	// rt is the RoundTripper to use, or nil for the cloud service default.
	rt http.RoundTripper
	// baseClient is a basic Google Storage client instance. It is used for
	// operations that don't need custom header injections.
	baseClient *gs.Client
}

// NewProdClient creates a new Client instance that uses production Cloud
// Storage.
//
// The supplied RoundTripper will be used to make connections. If nil, the
// default HTTP client will be used.
func NewProdClient(ctx context.Context, rt http.RoundTripper) (Client, error) {
	c := prodClient{
		Context: ctx,
		rt:      rt,
	}
	var err error
	c.baseClient, err = c.newClient()
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *prodClient) Close() error {
	return c.baseClient.Close()
}

func (c *prodClient) NewWriter(p Path) (Writer, error) {
	bucket, filename, err := splitPathErr(p)
	if err != nil {
		return nil, err
	}

	return &prodWriter{
		Context: c,
		client:  c,
		bucket:  bucket,
		relpath: filename,
	}, nil
}

func (c *prodClient) NewReader(p Path, offset, length int64) (io.ReadCloser, error) {
	if offset < 0 {
		panic(fmt.Errorf("offset (%d) must be >= 0", offset))
	}

	obj, err := c.handleForPath(p)
	if err != nil {
		return nil, err
	}
	return obj.NewRangeReader(c, offset, length)
}

func (c *prodClient) Rename(src, dst Path) error {
	srcObj, err := c.handleForPath(src)
	if err != nil {
		return fmt.Errorf("invalid source path: %s", err)
	}

	dstObj, err := c.handleForPath(dst)
	if err != nil {
		return fmt.Errorf("invalid destination path: %s", err)
	}

	// First stage: CopyTo
	err = retry.Retry(c, retry.TransientOnly(retry.Default), func() error {
		if _, err := srcObj.CopyTo(c, dstObj, nil); err != nil {
			// The storage library doesn't return gs.ErrObjectNotExist when Delete
			// returns a 404. Catch that explicitly.
			if isNotFoundError(err) {
				return err
			}

			// Assume all unexpected errors are transient.
			return errors.WrapTransient(err)
		}
		return nil
	}, func(err error, d time.Duration) {
		log.Fields{
			log.ErrorKey: err,
			"delay":      d,
			"src":        src,
			"dst":        dst,
		}.Warningf(c, "Transient error copying GS file. Retrying...")
	})
	if err != nil {
		return err
	}

	// Second stage: Delete. This is not fatal.
	if err := c.deleteObject(srcObj); err != nil {
		log.Fields{
			log.ErrorKey: err,
			"path":       src,
		}.Warningf(c, "(Non-fatal) Failed to delete source during rename.")
	}
	return nil
}

func (c *prodClient) Delete(p Path) error {
	dstObj, err := c.handleForPath(p)
	if err != nil {
		return fmt.Errorf("invalid path: %s", err)
	}

	return c.deleteObject(dstObj)
}

func (c *prodClient) deleteObject(o *gs.ObjectHandle) error {
	return retry.Retry(c, retry.TransientOnly(retry.Default), func() error {
		if err := o.Delete(c); err != nil {
			// The storage library doesn't return gs.ErrObjectNotExist when Delete
			// returns a 404. Catch that explicitly.
			if isNotFoundError(err) {
				// If the file wasn't found, then the delete "succeeded".
				return nil
			}

			// Assume all unexpected errors are transient.
			return errors.WrapTransient(err)
		}
		return nil
	}, func(err error, d time.Duration) {
		log.Fields{
			log.ErrorKey: err,
			"delay":      d,
		}.Warningf(c, "Transient error deleting file. Retrying...")
	})
}

func (c *prodClient) newClient() (*gs.Client, error) {
	var optsArray [1]option.ClientOption
	opts := optsArray[:0]
	if c.rt != nil {
		opts = append(opts, option.WithHTTPClient(&http.Client{
			Transport: c.rt,
		}))
	}
	return gs.NewClient(c, opts...)
}

func (c *prodClient) handleForPath(p Path) (*gs.ObjectHandle, error) {
	bucket, filename, err := splitPathErr(p)
	if err != nil {
		return nil, err
	}
	return c.baseClient.Bucket(bucket).Object(filename), nil
}

func splitPathErr(p Path) (bucket, filename string, err error) {
	bucket, filename = p.Split()
	switch {
	case bucket == "":
		err = errors.New("path has no bucket")
	case filename == "":
		err = errors.New("path has no filename")
	}
	return
}

func isNotFoundError(err error) bool {
	// The storage library doesn't return gs.ErrObjectNotExist when Delete
	// returns a 404. Catch that explicitly.
	if t, ok := err.(*googleapi.Error); ok {
		switch t.Code {
		case http.StatusNotFound:
			return true
		}
	}
	return false
}
