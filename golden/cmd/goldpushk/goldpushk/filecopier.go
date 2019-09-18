package goldpushk

import (
	"io/ioutil"

	"go.skia.org/infra/go/skerr"
)

// FileCopier defines an interface for copying files.
type FileCopier interface {
	// Copy copies a file from src to dst.
	Copy(src, dst string) error
}

// FileCopierImpl implements the FileCopier interface.
type FileCopierImpl struct{}

// Copy copies a file from src to dst.
func (fc *FileCopierImpl) Copy(src, dst string) error {
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return skerr.Wrap(err)
	}

	err = ioutil.WriteFile(dst, input, 0644)
	if err != nil {
		return skerr.Wrap(err)
	}

	return nil
}
