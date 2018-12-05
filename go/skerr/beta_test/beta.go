// This file contains helper functions for TestCallStack in skerr_test.go. Modifying this file may
// cause the test assertions to be incorrect.

package beta_test

import (
	"errors"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/skerr/alpha_test"
)

var (
	GenericError = errors.New("human detected")
)

func callAlphaInternal(alpha *alpha_test.Alpha) error {
	return alpha.Call()
}

func CallAlpha(alpha *alpha_test.Alpha) error {
	return callAlphaInternal(alpha)
}

func GetGenericError() error {
	return GenericError
}

func Context1(callback func() error) error {
	if err := callback(); err != nil {
		return skerr.Wrapf(err, "When searching for %d trees", 35)
	} else {
		return nil
	}
}

func Context2(callback func() error) error {
	if err := Context1(callback); err != nil {
		return skerr.Wrapf(err, "When walking the dog")
	} else {
		return nil
	}
}
