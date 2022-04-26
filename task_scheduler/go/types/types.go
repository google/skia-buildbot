package types

import "errors"

const (
	// The path of the git cookie file used by task scheduler. go/gitauth will
	// create and manage this file.
	GitCookiesPath = "/tmp/.gitcookies"
)

var (
	ErrUnknownId = errors.New("Unknown ID")
)
