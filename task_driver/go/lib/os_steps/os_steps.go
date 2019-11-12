package os_steps

/*
	Canned steps to be used for performing OS and filesystem interactions in task drivers.
*/

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"go.skia.org/infra/task_driver/go/td"
)

// Stat is a wrapper for os.Stat.
func Stat(ctx context.Context, path string) (os.FileInfo, error) {
	var rv os.FileInfo
	return rv, td.Do(ctx, td.Props(fmt.Sprintf("Stat %s", path)).Infra(), func(context.Context) error {
		fi, err := os.Stat(path)
		rv = fi
		return err
	})
}

// MkdirAll is a wrapper for os.MkdirAll.
func MkdirAll(ctx context.Context, path string) (err error) {
	return td.Do(ctx, td.Props(fmt.Sprintf("MkdirAll %s", path)).Infra(), func(context.Context) error {
		return os.MkdirAll(path, os.ModePerm)
	})
}

// RemoveAll is a wrapper for os.RemoveAll.
func RemoveAll(ctx context.Context, path string) (err error) {
	return td.Do(ctx, td.Props(fmt.Sprintf("RemoveAll %s", path)).Infra(), func(context.Context) error {
		return os.RemoveAll(path)
	})
}

// Abs is a wrapper for filepath.Abs.
func Abs(ctx context.Context, path string) (string, error) {
	var rv string
	err := td.Do(ctx, td.Props(fmt.Sprintf("Abs %s", path)).Infra(), func(context.Context) error {
		var err error
		rv, err = filepath.Abs(path)
		return err
	})
	return rv, err
}

// ReadFile is a wrapper for ioutil.ReadFile.
func ReadFile(ctx context.Context, path string) ([]byte, error) {
	var rv []byte
	err := td.Do(ctx, td.Props(fmt.Sprintf("Read %s", path)).Infra(), func(context.Context) error {
		var err error
		rv, err = ioutil.ReadFile(path)
		return err
	})
	return rv, err
}

// ReadDir is a wrapper for ioutil.ReadDir.
func ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	var rv []os.FileInfo
	err := td.Do(ctx, td.Props(fmt.Sprintf("ReadDir %s", path)).Infra(), func(context.Context) error {
		var err error
		rv, err = ioutil.ReadDir(path)
		return err
	})
	return rv, err
}

// Which returns the result of "which <exe>" (or "where <exe>" on Windows).
func Which(ctx context.Context, exe string) (string, error) {
	var rv string
	err := td.Do(ctx, td.Props(fmt.Sprintf("Which %s", exe)).Infra(), func(context.Context) error {
		var err error
		rv, err = exec.LookPath(exe)
		return err
	})
	return rv, err
}
