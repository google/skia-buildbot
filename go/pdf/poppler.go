package pdf

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"go.skia.org/infra/go/util"
)

const (
	pdftoppmExecutable = "pdftoppm" // provided by poppler-utils
	pnmtopngExecutable = "pnmtopng" // provided by netpbm
)

type Poppler struct{}

func (Poppler) String() string { return "Poppler" }

func (Poppler) Enabled() bool {
	return commandFound(pdftoppmExecutable) && commandFound(pnmtopngExecutable)
}

// This does the following:
//   `pdftoppm -r 72 -f 1 -l 1 < $PDF 2>/dev/null | pnmtopng 2> /dev/null > $PNG`
func (Poppler) Rasterize(pdfInputPath, pngOutputPath string) error {
	if !(Poppler{}).Enabled() {
		return fmt.Errorf("pdftoppm or pnmtopng is missing")
	}

	pdftoppm := exec.Command(pdftoppmExecutable, "-r", "72", "-f", "1", "-l", "1")
	pnmtopng := exec.Command(pnmtopngExecutable)

	defer processKiller(pdftoppm)
	defer processKiller(pnmtopng)

	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	defer fileCloser(pw)
	defer fileCloser(pr)
	pdftoppm.Stdout = pw
	pnmtopng.Stdin = pr

	iFile, err := os.Open(pdfInputPath)
	if err != nil {
		return err
	}
	defer util.Close(iFile)
	pdftoppm.Stdin = iFile

	oFile, err := os.Create(pngOutputPath)
	if err != nil {
		return err
	}
	defer util.Close(oFile)
	pnmtopng.Stdout = oFile

	if err := pdftoppm.Start(); err != nil {
		return err
	}
	if err := pnmtopng.Start(); err != nil {
		return err
	}

	go func() {
		time.Sleep(5 * time.Second)
		_ = pdftoppm.Process.Kill()
	}()
	if err := pdftoppm.Wait(); err != nil {
		return err
	}
	if err := pw.Close(); err != nil {
		return err
	}
	if err := pnmtopng.Wait(); err != nil {
		return err
	}
	return nil
}

// Prevents zombie processes
func processKiller(command *exec.Cmd) {
	if command.Process != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
	}
}

func fileCloser(c io.Closer) {
	_ = c.Close()
}
