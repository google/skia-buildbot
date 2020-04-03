// Package filter implements Accept/Reject filtering of file.Names.
package filter

import (
	"regexp"

	"go.skia.org/infra/go/skerr"
)

// Filter filenames by the accept/reject regexs.
type Filter struct {
	accept *regexp.Regexp
	reject *regexp.Regexp
}

// New returns a new *Filter.
//
// If accept is a non-empty regex string and it matches the filename then file
// will be processed. Leave the empty string to accept all files.
//
// If reject is a non-empty regex string and it matches the filename then the
// file will be ignored. Leave the empty string to disable rejection.
//
// If both regexs are set and either causes a file to be rejected then the file
// will be rejected.
func New(accept, reject string) (*Filter, error) {
	ret := &Filter{}

	if accept != "" {
		acceptRe, err := regexp.Compile(accept)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to parse regexp %q", accept)
		}
		ret.accept = acceptRe
	}
	if reject != "" {
		rejectRe, err := regexp.Compile(reject)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to parse regexp %q", reject)
		}
		ret.reject = rejectRe
	}
	return ret, nil
}

// Reject returns true if the file should be rejected based on its name.
func (f *Filter) Reject(name string) bool {
	if f.accept != nil && !f.accept.MatchString(name) {
		return true
	}
	if f.reject != nil && f.reject.MatchString(name) {
		return true
	}
	return false
}
