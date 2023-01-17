// Package common has functions useful across its peer modules.
package common

import (
	"context"
	"strings"
	"time"

	"go.skia.org/infra/go/executil"
)

const commandTimeout = 5 * time.Second

// TrimmedCommandOutput runs a command and returns its combined stdout and stdrerr
// (whitespace-trimmed), timing out after a period prescribed by commandTimeout. If the command
// returns a non-zero exit code, returned error is an exec.ExitError.
//
// idevice commands tend to return everything--both errors and normal output--on stderr. However,
// they don't advertise that as part of their contract, so we take both stdout and stderr for
// durability.
func TrimmedCommandOutput(ctx context.Context, commandName string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := executil.CommandContext(ctx, commandName, args...)
	outputBytes, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(outputBytes)), err // lop off newline
}
