// Package builders provides factory functions for Perf components.
//
// public_fs.go contains a wrapper around fs.FS that redirects requests for
// internal traces to the public trace bucket when configured to do so. It also
// implements a fallback mechanism to handle slight timestamp mismatches
// between identical traces in the internal and public buckets.
package builders

import (
	"context"
	"io/fs"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

type prefixFinder interface {
	FindFileByPrefix(ctx context.Context, namePrefix string) (string, error)
}

var timestampSuffixRegex = regexp.MustCompile(`_\d{4}_\d{2}_\d{2}_T\d{2}_\d{2}_\d{2}-UTC\.json$`)

type publicFS struct {
	fs.FS
	overrideGCS string
}

func (r *publicFS) Open(name string) (fs.File, error) {
	if !strings.HasPrefix(name, "gs://") {
		file, err := r.FS.Open(name)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return file, nil
	}

	u, err := url.Parse(name)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	u.Host = r.overrideGCS
	rewrittenName := u.String()

	f, err := r.FS.Open(rewrittenName)
	if err != nil {
		// Fallback: If exact match failed, try to find a file with the same prefix (ignoring the timestamp).
		if pf, ok := r.FS.(prefixFinder); ok {
			prefix := timestampSuffixRegex.ReplaceAllString(rewrittenName, "")
			if prefix != rewrittenName {
				// We use a hardcoded timeout with context.Background() here because the standard
				// fs.FS interface does not support passing a context into Open(). This prevents
				// the GCS fallback search from hanging indefinitely if there are network issues.
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if actualName, findErr := pf.FindFileByPrefix(ctx, prefix); findErr == nil {
					return r.FS.Open(actualName)
				} else {
					sklog.Warningf("Fallback prefix search for %q failed: %v", prefix, findErr)
				}
			}
		}
		return nil, err
	}

	return f, nil
}
