package os_steps

/*
	Canned steps to be used for performing OS and filesystem interactions in task drivers.
*/

import (
	"context"
	"fmt"
	"os"

	"go.skia.org/infra/go/task_driver"
)

// Stat is a wrapper for os.Stat.
func Stat(ctx context.Context, path string) (os.FileInfo, error) {
	var rv os.FileInfo
	return rv, task_driver.Do(ctx, task_driver.Opts(task_driver.Infra(), task_driver.Name(fmt.Sprintf("Stat %s", path))), func(context.Context) error {
		fi, err := os.Stat(path)
		rv = fi
		return err
	})
}

// MkdirAll is a wrapper for os.MkdirAll.
func MkdirAll(ctx context.Context, path string) (err error) {
	return task_driver.Do(ctx, task_driver.Opts(task_driver.Infra(), task_driver.Name(fmt.Sprintf("MkdirAll %s", path))), func(context.Context) error {
		return os.MkdirAll(path, os.ModePerm)
	})
}

// RemoveAll is a wrapper for os.RemoveAll.
func RemoveAll(ctx context.Context, path string) (err error) {
	return task_driver.Do(ctx, task_driver.Opts(task_driver.Infra(), task_driver.Name(fmt.Sprintf("RemoveAll %s", path))), func(context.Context) error {
		return os.RemoveAll(path)
	})
}
