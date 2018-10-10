package os_steps

/*
	Canned steps to be used for performing OS and filesystem interactions in task drivers.
*/

import (
	"fmt"
	"os"

	"go.skia.org/infra/go/task_driver"
)

// Stat is a wrapper for os.Stat.
func Stat(s *task_driver.Step, path string) (os.FileInfo, error) {
	var rv os.FileInfo
	return rv, s.Step().Infra().Name(fmt.Sprintf("Stat %s", path)).Do(func(*task_driver.Step) error {
		fi, err := os.Stat(path)
		rv = fi
		return err
	})
}

// MkdirAll is a wrapper for os.MkdirAll.
func MkdirAll(s *task_driver.Step, path string) (err error) {
	return s.Step().Infra().Name(fmt.Sprintf("MkdirAll %s", path)).Do(func(*task_driver.Step) error {
		return os.MkdirAll(path, os.ModePerm)
	})
}

// RemoveAll is a wrapper for os.RemoveAll.
func RemoveAll(s *task_driver.Step, path string) (err error) {
	return s.Step().Infra().Name(fmt.Sprintf("RemoveAll %s", path)).Do(func(*task_driver.Step) error {
		return os.RemoveAll(path)
	})
}
