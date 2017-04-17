package common

import (
	"fmt"
	"os"
)

// The CommonImpl allows for unit tests to mock out some functions in
// the common package.
type CommonImpl interface {
	Hostname() string
}

type defaultImpl struct{}

func (d *defaultImpl) Hostname() string {
	if h, err := os.Hostname(); err != nil {
		// Can't use sklog or we get a dependency loop
		return fmt.Sprintf("HOSTNAME ERROR %s", err)
	} else {
		return h
	}
}
