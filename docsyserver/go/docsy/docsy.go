// Package docsy transforms raw documents via Hugo and a Docsy template into
// final documentation.
package docsy

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
)

// Docsy renders documentation against a Docsy template.
type Docsy interface {
	// Render the documentation in 'src' into 'dst' using Hugo and the Docsy
	// templates.
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

	// renderMetric records how long it is taking hugo to run.
	renderMetric metrics2.Float64SummaryMetric
}

// New returns an instance of *docsy.
func New(hugoExe string, docsyDir string, docPath string) *docsy {
	return &docsy{
		hugoExe:      hugoExe,
		docsyDir:     docsyDir,
		docPath:      docPath,
		renderMetric: metrics2.GetFloat64SummaryMetric("docserver_docsy_render"),
	}
}

// Render implements Docsy.
func (d *docsy) Render(ctx context.Context, src, dst string) error {
	defer timer.NewWithSummary("docsy_render", d.renderMetric).Stop()
	cmd := executil.CommandContext(ctx,
		d.hugoExe,
		fmt.Sprintf("--source=%s", d.docsyDir),
		fmt.Sprintf("--destination=%s", dst),
		fmt.Sprintf("--config=%s", filepath.Join(src, "config.toml")),
		fmt.Sprintf("--contentDir=%s", src),
	)
	sklog.Info(cmd.String())

	b, err := cmd.Output()

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			err = skerr.Wrapf(err, "hugo stderr: %q\n stdout: %q", ee.Stderr, string(b))
		}
		return err
	}
	return nil
}

// Assert that docsy implements Docsy.
var _ Docsy = (*docsy)(nil)
