// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cipd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/logging"
)

const (
	// uploadChunkSize is the max size of a single PUT HTTP request during upload.
	uploadChunkSize int64 = 4 * 1024 * 1024
	// uploadMaxErrors is the number of transient errors after which upload is aborted.
	uploadMaxErrors = 20
	// downloadReportInterval defines frequency of "downloaded X%" log lines.
	downloadReportInterval = 5 * time.Second
	// downloadMaxAttempts is how many times to retry a download on errors.
	downloadMaxAttempts = 10
)

// errTransientError is returned by getNextOffset in case of retryable error.
var errTransientError = errors.New("Transient error in getUploadedOffset")

// storageImpl implements storage via Google Storage signed URLs.
type storageImpl struct {
	chunkSize int64
	userAgent string
	client    *http.Client
}

// Google Storage resumable upload protocol.
// See https://cloud.google.com/storage/docs/concepts-techniques#resumable

func (s *storageImpl) upload(ctx context.Context, url string, data io.ReadSeeker) error {
	// Grab the total length of the file.
	length, err := data.Seek(0, os.SEEK_END)
	if err != nil {
		return err
	}
	_, err = data.Seek(0, os.SEEK_SET)
	if err != nil {
		return err
	}

	// Offset of data known to be persisted in the storage or -1 if needed to ask
	// Google Storage for it.
	offset := int64(0)
	// Number of transient errors reported so far.
	errors := 0

	// Called when some new data is uploaded.
	reportProgress := func(offset int64) {
		if length != 0 {
			logging.Infof(ctx, "cipd: uploading - %d%%", offset*100/length)
		}
	}

	// Called when transient error happens.
	reportTransientError := func() {
		// Need to query for latest uploaded offset to resume.
		errors++
		offset = -1
		if err := ctx.Err(); err == nil {
			if errors < uploadMaxErrors {
				logging.Warningf(ctx, "cipd: transient upload error, retrying...")
				clock.Sleep(ctx, 2*time.Second)
			}
		}
	}

	reportProgress(0)
	for errors < uploadMaxErrors {
		// Context canceled?
		if err := ctx.Err(); err != nil {
			return err
		}

		// Grab latest uploaded offset if not known.
		if offset == -1 {
			offset, err = s.getNextOffset(ctx, url, length)
			if err == errTransientError {
				reportTransientError()
				continue
			}
			if err != nil {
				return err
			}
			reportProgress(offset)
			if offset == length {
				return nil
			}
			logging.Warningf(ctx, "cipd: resuming upload from offset %d", offset)
		}

		// Length of a chunk to upload.
		chunk := s.chunkSize
		if chunk > length-offset {
			chunk = length - offset
		}

		// Upload the chunk.
		data.Seek(offset, os.SEEK_SET)
		r, err := http.NewRequest("PUT", url, io.LimitReader(data, chunk))
		if err != nil {
			return err
		}
		rangeHeader := fmt.Sprintf("bytes %d-%d/%d", offset, offset+chunk-1, length)
		r.Header.Set("Content-Range", rangeHeader)
		r.Header.Set("Content-Length", fmt.Sprintf("%d", chunk))
		r.Header.Set("User-Agent", s.userAgent)
		resp, err := ctxhttp.Do(ctx, s.client, r)
		if err != nil {
			if isTemporaryNetError(err) {
				reportTransientError()
				continue
			}
			return err
		}
		resp.Body.Close()

		// Partially or fully uploaded.
		if resp.StatusCode == 308 || resp.StatusCode == 200 {
			offset += chunk
			reportProgress(offset)
			if offset == length {
				return nil
			}
			continue
		}

		// Transient error.
		if isTemporaryHTTPError(resp.StatusCode) {
			reportTransientError()
			continue
		}

		// Fatal error.
		return fmt.Errorf("unexpected response during file upload: HTTP %d", resp.StatusCode)
	}

	return ErrUploadError
}

// getNextOffset queries the storage for size of persisted data.
func (s *storageImpl) getNextOffset(ctx context.Context, url string, length int64) (offset int64, err error) {
	r, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return
	}
	r.Header.Set("Content-Range", fmt.Sprintf("bytes */%d", length))
	r.Header.Set("Content-Length", "0")
	r.Header.Set("User-Agent", s.userAgent)
	resp, err := ctxhttp.Do(ctx, s.client, r)
	if err != nil {
		if isTemporaryNetError(err) {
			err = errTransientError
		}
		return
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		// Fully uploaded.
		offset = length
	} else if resp.StatusCode == 308 {
		// Partially uploaded, extract last uploaded offset from Range header.
		rangeHeader := resp.Header.Get("Range")
		if rangeHeader != "" {
			_, err = fmt.Sscanf(rangeHeader, "bytes=0-%d", &offset)
			if err == nil {
				// |offset| is an *offset* of a last uploaded byte, not the data length.
				offset++
			}
		}
	} else if isTemporaryHTTPError(resp.StatusCode) {
		err = errTransientError
	} else {
		err = fmt.Errorf("unexpected response (HTTP %d) when querying for uploaded offset", resp.StatusCode)
	}
	return
}

// TODO(vadimsh): Use resumable download protocol.

func (s *storageImpl) download(ctx context.Context, url string, output io.WriteSeeker) error {
	// reportProgress print fetch progress, throttling the reports rate.
	var prevProgress int64 = 1000 // >100%
	var prevReportTs time.Time
	reportProgress := func(read int64, total int64) {
		now := clock.Now(ctx)
		progress := read * 100 / total
		if progress < prevProgress || read == total || now.Sub(prevReportTs) > downloadReportInterval {
			logging.Infof(ctx, "cipd: fetching - %d%%", progress)
			prevReportTs = now
			prevProgress = progress
		}
	}

	// download is a separate function to be able to use deferred close.
	download := func(out io.WriteSeeker, src io.ReadCloser, totalLen int64) error {
		defer src.Close()
		logging.Infof(ctx, "cipd: about to fetch %.1f Mb", float32(totalLen)/1024.0/1024.0)
		reportProgress(0, totalLen)
		_, err := io.Copy(out, &readerWithProgress{
			reader:   src,
			callback: func(read int64) { reportProgress(read, totalLen) },
		})
		if err == nil {
			logging.Infof(ctx, "cipd: fetch finished successfully")
		}
		return err
	}

	// reportTransientError logs the error and sleep few seconds.
	reportTransientError := func(msg string, args ...interface{}) {
		if err := ctx.Err(); err != nil {
			return
		}
		logging.Warningf(ctx, msg, args...)
		clock.Sleep(ctx, 2*time.Second)
	}

	// Download the actual data (several attempts).
	for attempt := 0; attempt < downloadMaxAttempts; attempt++ {
		// Context canceled?
		if err := ctx.Err(); err != nil {
			return err
		}

		// Rewind output to zero offset.
		_, err := output.Seek(0, os.SEEK_SET)
		if err != nil {
			return err
		}

		// Send the request.
		logging.Infof(ctx, "cipd: initiating the fetch")
		var req *http.Request
		var resp *http.Response
		req, err = http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", s.userAgent)
		resp, err = ctxhttp.Do(ctx, s.client, req)
		if err != nil {
			if isTemporaryNetError(err) {
				reportTransientError("cipd: transient network error: %s", err)
				continue
			}
			return err
		}

		// Transient error, retry.
		if isTemporaryHTTPError(resp.StatusCode) {
			resp.Body.Close()
			reportTransientError("cipd: transient HTTP error %d while fetching the file", resp.StatusCode)
			continue
		}

		// Fatal error, abort.
		if resp.StatusCode >= 400 {
			resp.Body.Close()
			return fmt.Errorf("server replied with HTTP code %d", resp.StatusCode)
		}

		// Try to fetch (will close resp.Body when done).
		err = download(output, resp.Body, resp.ContentLength)
		if err != nil {
			reportTransientError("cipd: transient error fetching the file: %s", err)
			continue
		}

		// Success.
		return nil
	}

	return ErrDownloadError
}

// readerWithProgress is io.Reader that calls callback whenever something is
// read from it.
type readerWithProgress struct {
	reader   io.Reader
	total    int64
	callback func(total int64)
}

func (r *readerWithProgress) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.total += int64(n)
	r.callback(r.total)
	return n, err
}
