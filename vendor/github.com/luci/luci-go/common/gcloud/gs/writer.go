// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gs

import (
	"io"
	"time"

	gs "cloud.google.com/go/storage"
	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/retry"
	"golang.org/x/net/context"
)

// Writer is an augmented io.WriteCloser instance.
type Writer interface {
	io.WriteCloser

	// Count returns the number of bytes written by the object.
	Count() int64
}

type prodWriter struct {
	context.Context

	// Writer is the active Writer instance. It will be nil until the first Write
	// invocation.
	*gs.Writer

	client  *prodClient
	bucket  string
	relpath string
	count   int64
}

// Write writes data with exponenital backoff/retry.
func (w *prodWriter) Write(d []byte) (a int, err error) {
	if w.Writer == nil {
		w.Writer = w.client.baseClient.Bucket(w.bucket).Object(w.relpath).NewWriter(w)
	}

	err = retry.Retry(w, retry.TransientOnly(retry.Default), func() (ierr error) {
		a, ierr = w.Writer.Write(d)

		// Assume all Write errors are transient.
		ierr = errors.WrapTransient(ierr)
		return
	}, func(err error, d time.Duration) {
		log.Fields{
			log.ErrorKey: err,
			"delay":      d,
			"bucket":     w.bucket,
			"path":       w.relpath,
		}.Warningf(w, "Transient error on GS write. Retrying...")
	})

	w.count += int64(a)
	return
}

func (w *prodWriter) Close() error {
	if w.Writer == nil {
		return nil
	}

	return retry.Retry(w, retry.TransientOnly(retry.Default),
		w.Writer.Close,
		func(err error, d time.Duration) {
			log.Fields{
				log.ErrorKey: err,
				"delay":      d,
				"bucket":     w.bucket,
				"path":       w.relpath,
			}.Warningf(w, "Transient error closing GS Writer. Retrying...")
		})
}

func (w *prodWriter) Count() int64 {
	return w.count
}
