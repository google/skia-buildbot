// Package docsy transforms raw documents via Hugo and a Docsy template into
// final documentation.
package docsy

import (
	"context"
	"fmt"
	"path/filepath"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/skerr"
)

// Docsy take an input directory and renders HTML/CSS into the destination directory.
type Docsy interface {
	Render(ctx context.Context, src, dst string) error
}

// docsy implements Docsy.
type docsy struct {
	// Absolute path the 'hugo' executable.
	hugoExe string

	// The directory where Docsy is located.
	docsyDir string

	// The relative path in the git repo where the docs are stored, e.g. "site"
	// for Skia.
	docPath string
}

// New returns an instance of *docsy.
func New(hugoExe string, docsyDir string, docPath string) *docsy {
	return &docsy{
		hugoExe:  hugoExe,
		docsyDir: docsyDir,
		docPath:  docPath,
	}
}

// Render implements Docsy.
func (d *docsy) Render(ctx context.Context, src, dst string) error {
	cmd := executil.CommandContext(ctx,
		d.hugoExe,
		fmt.Sprintf("--source=%s", d.docsyDir),
		fmt.Sprintf("--destination=%s", dst),
		fmt.Sprintf("--config=%s", filepath.Join(d.docPath, "config.toml")),
		fmt.Sprintf("--contentDir=%s", src),
	)

	b, err := cmd.Output()
	if err != nil {
		err = skerr.Wrapf(err, "hugo with stdout: %q", string(b))
		return err
	}
	return nil
}

// Assert that docsy implements Docsy.
var _ Docsy = (*docsy)(nil)
