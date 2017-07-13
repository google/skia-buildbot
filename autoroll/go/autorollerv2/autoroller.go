package autorollerv2

import (
	"context"
	"path"
	"sync"
	"time"

	"go.skia.org/infra/autoroll/go/autoroll_modes"
	"go.skia.org/infra/autoroll/go/autoroller"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// AutoRoller is a struct which automates the merging new revisions of one
// project into another.
type AutoRoller struct {
	cqExtraTrybots  string
	currentRoll     state_machine.RollCLImpl
	emails          []string
	emailsMtx       sync.RWMutex
	gerrit          *gerrit.Gerrit
	lastError       error
	liveness        metrics2.Liveness
	modeHistory     *autoroll_modes.ModeHistory
	modeMtx         sync.Mutex
	recent          *recent_rolls.RecentRolls
	retrieveRoll    func(int64) (state_machine.RollCLImpl, error)
	rm              repo_manager.RepoManager
	runningMtx      sync.Mutex
	sm              *state_machine.AutoRollStateMachine
	status          *autoroller.AutoRollStatusCache
	rollIntoAndroid bool
}

// newAutoRoller returns an AutoRoller instance.
func newAutoRoller(workdir, childPath, cqExtraTrybots string, emails []string, gerrit *gerrit.Gerrit, rm repo_manager.RepoManager, retrieveRoll func(int64) (state_machine.RollCLImpl, error)) (*AutoRoller, error) {
	recent, err := recent_rolls.NewRecentRolls(path.Join(workdir, "recent_rolls.db"))
	if err != nil {
		return nil, err
	}

	mh, err := autoroll_modes.NewModeHistory(path.Join(workdir, "autoroll_modes.db"))
	if err != nil {
		return nil, err
	}

	arb := &AutoRoller{
		cqExtraTrybots: cqExtraTrybots,
		emails:         emails,
		gerrit:         gerrit,
		liveness:       metrics2.NewLiveness("last-autoroll-landed", map[string]string{"child-path": childPath}),
		modeHistory:    mh,
		recent:         recent,
		retrieveRoll:   retrieveRoll,
		rm:             rm,
		status:         &autoroller.AutoRollStatusCache{},
	}
	sm, err := state_machine.New(arb, workdir)
	if err != nil {
		return nil, err
	}
	arb.sm = sm
	current := recent.CurrentRoll()
	if current != nil {
		roll, err := arb.retrieveRoll(current.Issue)
		if err != nil {
			return nil, err
		}
		arb.currentRoll = roll
	}
	return arb, nil
}

// NewAndroidAutoRoller returns an AutoRoller instance which rolls into Android.
func NewAndroidAutoRoller(workdir, parentBranch, childPath, childBranch, cqExtraTrybots string, emails []string, gerrit *gerrit.Gerrit, strategy string) (*AutoRoller, error) {
	rm, err := repo_manager.NewAndroidRepoManager(workdir, parentBranch, childPath, childBranch, gerrit, strategy)
	if err != nil {
		return nil, err
	}
	var arb *AutoRoller
	retrieveRoll := func(issue int64) (state_machine.RollCLImpl, error) {
		return newGerritAndroidRoll(arb.gerrit, arb.rm, arb.recent, issue)
	}
	arb, err = newAutoRoller(workdir, childPath, cqExtraTrybots, emails, gerrit, rm, retrieveRoll)
	if err != nil {
		return nil, err
	}
	return arb, nil
}

// NewDEPSAutoRoller returns an AutoRoller instance which rolls using DEPS.
func NewDEPSAutoRoller(workdir, parentRepo, parentBranch, childPath, childBranch, cqExtraTrybots string, emails []string, gerrit *gerrit.Gerrit, depot_tools string, strategy string) (*AutoRoller, error) {
	rm, err := repo_manager.NewDEPSRepoManager(workdir, parentRepo, parentBranch, childPath, childBranch, depot_tools, gerrit, strategy)
	if err != nil {
		return nil, err
	}
	var arb *AutoRoller
	retrieveRoll := func(issue int64) (state_machine.RollCLImpl, error) {
		return newGerritRoll(arb.gerrit, arb.rm, arb.recent, issue)
	}
	arb, err = newAutoRoller(workdir, childPath, cqExtraTrybots, emails, gerrit, rm, retrieveRoll)
	if err != nil {
		return nil, err
	}
	return arb, nil
}

// Start initiates the AutoRoller's loop.
func (r *AutoRoller) Start(tickFrequency, repoFrequency time.Duration, ctx context.Context) {
	repo_manager.Start(r.rm, repoFrequency, ctx)
	lv := metrics2.NewLiveness("last-successful-autoroll-tick")
	go util.RepeatCtx(tickFrequency, ctx, func() {
		if err := r.Tick(); err != nil {
			sklog.Errorf("Failed to run autoroll: %s", err)
		} else {
			lv.Reset()
		}
	})
	go func() {
		for {
			select {
			case <-ctx.Done():
				util.LogErr(r.recent.Close())
				util.LogErr(r.modeHistory.Close())
			default:
			}
		}
	}()
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) GetActiveRoll() state_machine.RollCLImpl {
	return r.currentRoll
}

// GetEmails returns the list of email addresses which are copied on rolls.
func (r *AutoRoller) GetEmails() []string {
	r.emailsMtx.RLock()
	defer r.emailsMtx.RUnlock()
	rv := make([]string, len(r.emails))
	copy(rv, r.emails)
	return rv
}

// SetEmails sets the list of email addresses which are copied on rolls.
func (r *AutoRoller) SetEmails(e []string) {
	r.emailsMtx.Lock()
	defer r.emailsMtx.Unlock()
	emails := make([]string, len(e))
	copy(emails, e)
	r.emails = emails
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) GetMode() string {
	r.modeMtx.Lock()
	defer r.modeMtx.Unlock()
	return r.modeHistory.CurrentMode().Mode
}

// SetMode sets the desired mode of the bot. This forces the bot to run and
// blocks until it finishes.
func (r *AutoRoller) SetMode(m, user, message string) error {
	r.modeMtx.Lock()
	defer r.modeMtx.Unlock()
	if err := r.modeHistory.Add(m, user, message); err != nil {
		return err
	}
	return r.Tick()
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) UploadNewRoll(from, to string, dryRun bool) error {
	issueNum, err := r.rm.CreateNewRoll(from, to, r.GetEmails(), r.cqExtraTrybots, dryRun)
	if err != nil {
		return err
	}
	roll, err := r.retrieveRoll(issueNum)
	if err != nil {
		return err
	}
	r.currentRoll = roll
	return nil
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) GetCurrentRev() string {
	return r.rm.LastRollRev()
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) GetNextRollRev() string {
	return r.rm.NextRollRev()
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) RolledPast(rev string) (bool, error) {
	return r.rm.RolledPast(rev)
}

// See documentation for state_machine.AutoRollerImpl interface.
func (r *AutoRoller) UpdateRepos() error {
	return r.rm.Update()
}

// Run one iteration of the roller.
func (r *AutoRoller) Tick() error {
	// Run the state machine.
	lastErr := r.sm.NextTransitionSequence()

	// Update the status information.
	lastErrorStr := ""
	if lastErr != nil {
		lastErrorStr = lastErr.Error()
	}
	if err := r.status.Set(&autoroller.AutoRollStatus{
		CurrentRoll: r.recent.CurrentRoll(),
		Error:       lastErrorStr,
		GerritUrl:   r.gerrit.Url(0),
		LastRoll:    r.recent.LastRoll(),
		LastRollRev: r.rm.LastRollRev(),
		Mode:        r.modeHistory.CurrentMode(),
		Recent:      r.recent.GetRecentRolls(),
		Status:      string(r.sm.Current()),
	}); err != nil {
		return err
	}
	return lastErr
}
