package trace_processor_common

import (
	"context"
	"path/filepath"
	"strings"

	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/skerr"
)

// TODO(borenet): This is going to be extremely slow.
const RepoURL = "https://chromium.googlesource.com/chromium/src.git"

type traceProcessorRepoManager struct {
	branch   string
	co       *git.Checkout
	dir      string
	platform string
}

// NewTraceProcessorRepoManager returns a RepoManager implementation which rolls
// trace_processor_shell into Chrome.
func NewTraceProcessorRepoManager(ctx context.Context, workdir string) (*traceProcessorRepoManager, error) {
	co, err := git.NewCheckout(ctx, RepoURL, workdir)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &traceProcessorRepoManager{
		branch:   "master",
		co:       co,
		dir:      filepath.Join(co.Dir(), "tools", "perf", "core", "perfetto_binary_roller"),
		platform: "linux",
	}, nil
}

// run roll_trace_processor with the given args and return the output.
func (m *traceProcessorRepoManager) run(ctx context.Context, args ...string) (string, error) {
	cmd = append([]string{"./roll_trace_processor", "--platform", m.platform}, args...)
	out, err := exec.RunCwd(ctx, m.dir, "./roll_trace_processor", args...)
	if err != nil {
		return out, err
	}
	return strings.TrimSpace(out, nil)
}

// See documentation for RepoManager interface.
func (m *traceProcessorRepoManager) Update(ctx context.Context) (*revision.Revision, *revision.Revision, []*revision.Revision, error) {
	if err := m.co.UpdateBranch(ctx, m.branch); err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	tipRevStr, err := m.run(ctx, "--print-latest")
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	tipRev := &revision.Revision{Id: tipRevStr}
	lastRollRevStr, err := m.run(ctx, "--print-current")
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	lastRollRev := &revision.Revision{Id: lastRollRevStr}
	var notRolledRevs []*revision.Revision
	if lastRollRevStr != tipRevStr {
		notRolledRevs = append(notRolledRevs, tipRev)
	}
	return lastRollRev, tipRev, notRolledRevs, nil
}

// See documentation for RepoManager interface.
func (m *traceProcessorRepoManager) GetRevision(ctx context.Context, id string) (*revision.Revision, error) {
	return &revision.Revision{Id: id}, nil
}

// See documentation for RepoManager interface.
func (m *traceProcessorRepoManager) CreateNewRoll(ctx context.Context, from, to *revision.Revision, rolling []*revision.Revision, emails []string, cqExtraTrybots string, dryRun bool) (int64, error) {
	return 0, skerr.Fmt("TODO")
}

// traceProcessorRepoManager implements RepoManager.
var _ RepoManager = &traceProcessorRepoManager{}
