// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package annotation

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/proto/google"
	"github.com/luci/luci-go/common/proto/milo"
	"github.com/luci/luci-go/logdog/common/types"
)

// UpdateType is information sent to the Updated callback to indicate the nature
// of the update.
type UpdateType int

const (
	// UpdateIterative indicates that a non-structural update occurred.
	UpdateIterative UpdateType = iota
	// UpdateStructural indicates that a structural update has occurred. A
	// structural update is one that affects the existence of or relationship of
	// the Steps in the annotation.
	UpdateStructural
)

// Callbacks is the set of callbacks that a State may invoke as it processes
// annotations.
type Callbacks interface {
	// StepClosed is called when a Step has closed. An Updated callback will still
	// be invoked.
	StepClosed(*Step)
	// Updated is called when a Step's state has been updated.
	Updated(*Step, UpdateType)
	// StepLogLine is called when a Step emits a log line.
	StepLogLine(s *Step, stream types.StreamName, label, line string)
	// StepLogEnd is called when a Step finishes emitting logs.
	StepLogEnd(*Step, types.StreamName)
}

// State is the aggregate annotation state for a given annotation
// stream. It receives updates in the form of annotations added via Append,
// and can be serialized to a Milo annotation state protobuf.
type State struct {
	// LogNameBase is the base log stream name that is prepeneded to generated
	// log streams.
	LogNameBase types.StreamName
	// Callbacks implements annotation callbacks. It may not be nil.
	Callbacks Callbacks
	// Execution is the supplied Execution. If nil, no execution details will be
	// added to the generated Milo protos.
	Execution *Execution
	// Offline specifies whether parsing happens not at the same time as
	// emitting. If true and CURRENT_TIMESTAMP annotations are not provided
	// then step start/end times are left empty.
	Offline bool
	// Clock is the clock implementation to use for time information.
	// Defaults to system time.
	Clock clock.Clock

	// stepMap is a map of step name to Step instance.
	//
	// If stepMap is nil, the State is considered uninitialized.
	stepMap    map[string]*Step
	latestStep *Step
	rootStep   Step
	// stepCursor is the current cursor step name. This will always point to a
	// valid Step, falling back to rootStep.
	stepCursor *Step
	// startedProcessing is true iff processed at least one annotation.
	startedProcessing bool

	// stepLookup is a mapping of *milo.Step entries to their respective *Step
	// entries.
	stepLookup map[*milo.Step]*Step

	// currentTimestamp is time for the next annotation expected in Append.
	currentTimestamp *google.Timestamp
	closed           bool
	haltOnFailure    bool
}

// initialize sets of the State's initial state. It will execute exactly once,
// and must be called by any State methods that access internal variables.
func (s *State) initialize() {
	if s.stepMap != nil {
		return
	}

	s.stepMap = map[string]*Step{}
	s.stepLookup = map[*milo.Step]*Step{}

	name := "steps"
	if s.Execution != nil {
		name = s.Execution.Name
	}
	s.rootStep.initializeStep(s, nil, name, 0, s.LogNameBase)
	s.SetCurrentStep(nil)

	// Add our Command parameters, if applicable.
	if exec := s.Execution; exec != nil {
		s.rootStep.Command = &milo.Step_Command{
			CommandLine: exec.Command,
			Cwd:         exec.Dir,
			Environ:     exec.Env,
		}
	}

	var annotatedNow *google.Timestamp
	if !s.Offline {
		annotatedNow = s.now()
	}
	s.rootStep.Start(annotatedNow)
}

// Append adds an annotation to the state. If the state was updated, Append will
// return true.
//
// The appended annotation should only contain the annotation text body, not any
// annotation indicators (e.g., "@@@") that surround it.
//
// If the annotation is invalid or could not be added to the state, an error
// will be returned.
//
// Steps and descriptions can be found at:
// https://chromium.googlesource.com/chromium/tools/build/+/master/scripts/
// master/chromium_step.py
func (s *State) Append(annotation string) error {
	s.initialize()

	firstAnnotation := !s.startedProcessing
	s.startedProcessing = true

	command, params := annotation, ""
	splitIdx := strings.IndexAny(command, "@ ")
	if splitIdx > 0 {
		command, params = command[:splitIdx], command[splitIdx+1:]
	}

	if s.closed {
		return nil
	}

	var (
		updated    *Step
		updateType UpdateType
	)
	updatedIf := func(s *Step, u UpdateType, b bool) {
		if b {
			updated, updateType = s, u
		}
	}

	annotatedNow := s.currentTimestamp
	s.currentTimestamp = nil
	if annotatedNow == nil && !s.Offline {
		annotatedNow = s.now()
	}

	switch command {
	// @@@CURRENT_TIMESTAMP@unix_timestamp@@@
	case "CURRENT_TIMESTAMP":
		// This annotation is printed at the beginning and end of the
		// stream, as well as before each STEP_STARTED and STEP_CLOSED
		// annotations. It effectively specifies step start/end times,
		// including root step.
		timestamp, err := strconv.ParseFloat(params, 64)
		if err != nil {
			return fmt.Errorf("CURRENT_TIMESTAMP parameter %q is not a number: %s", params, err)
		}
		s.currentTimestamp = google.NewTimestamp(time.Unix(
			int64(timestamp),
			int64(timestamp*1000000000)%1000000000))
		if firstAnnotation {
			s.rootStep.Started = s.currentTimestamp
		}

	// @@@BUILD_STEP <stepname>@@@
	case "BUILD_STEP":
		// Close the last section.
		step := s.CurrentStep()
		if step != nil && step != s.RootStep() {
			if step.Name() == params {
				// Same step; ignore the command.
				break
			}
			step.Close(annotatedNow)
		}

		step = s.rootStep.AddStep(params)
		step.Start(annotatedNow)
		s.SetCurrentStep(step)
		updatedIf(step, UpdateStructural, true)

	//  @@@SEED_STEP <stepname>@@@
	case "SEED_STEP":
		step := s.LookupStep(params)
		if step == nil {
			step = s.rootStep.AddStep(params)
			updatedIf(step, UpdateIterative, true)
		}

	// @@@STEP_CURSOR <stepname>@@@
	case "STEP_CURSOR":
		step, err := s.LookupStepErr(params)
		if err != nil {
			return fmt.Errorf("STEP_CURSOR could not lookup step: %s", err)
		}
		s.SetCurrentStep(step)

	// @@@STEP_LINK@<label>@<url>@@@
	case "link":
		fallthrough
	case "STEP_LINK":
		step := s.CurrentStep()
		parts := strings.SplitN(params, "@", 2)
		if len(parts) != 2 {
			return fmt.Errorf("STEP_LINK link [%s] missing URL", parts[0])
		}

		// If if link is an alias, parse it as one.
		alias := strings.SplitN(parts[0], "-->", 2)
		if len(alias) == 2 && len(alias[0]) > 0 && len(alias[1]) > 0 {
			// parrts[0] is an alias of the form: "text-->base"
			step.AddURLLink(alias[1], alias[0], parts[1])
		} else {
			step.AddURLLink(parts[0], "", parts[1])
		}
		updatedIf(step, UpdateIterative, true)

	//  @@@STEP_STARTED@@@
	case "STEP_STARTED":
		step := s.CurrentStep()
		updatedIf(step, UpdateIterative, step.Start(annotatedNow))

	//  @@@STEP_WARNINGS@@@
	case "BUILD_WARNINGS":
		fallthrough
	case "STEP_WARNINGS":
		// No warnings because they don't generally help. Builds that want to add
		// information can do so with notes. A "WARNING" state is traditionally a
		// success state with a call to attention, and that call can be done through
		// other means.
		break

	// @@@STEP_FAILURE@@@
	case "BUILD_FAILED":
		fallthrough
	case "STEP_FAILURE":
		step := s.CurrentStep()
		updatedIf(step, UpdateIterative, step.SetStatus(milo.Status_FAILURE, nil))
		if s.haltOnFailure {
			updatedIf(step, UpdateIterative, s.finishWithStatus(milo.Status_FAILURE, nil))
		}

	// @@@STEP_EXCEPTION@@@
	case "BUILD_EXCEPTION":
		fallthrough
	case "STEP_EXCEPTION":
		step := s.CurrentStep()
		updatedIf(step, UpdateIterative, step.SetStatus(milo.Status_FAILURE, &milo.FailureDetails{
			Type: milo.FailureDetails_EXCEPTION,
		}))

		// @@@STEP_CLOSED@@@
	case "STEP_CLOSED":
		step := s.CurrentStep()
		updatedIf(step, UpdateStructural, step.Close(annotatedNow))

	// @@@STEP_LOG_LINE@<label>@<line>@@@
	case "STEP_LOG_LINE":
		step := s.CurrentStep()

		parts := strings.SplitN(params, "@", 2)
		label, line := parts[0], ""
		if len(parts) == 2 {
			line = parts[1]
		}
		updatedIf(step, UpdateIterative, step.LogLine(label, line))

		// @@@STEP_LOG_END@<label>@@@
	case "STEP_LOG_END":
		s.CurrentStep().LogEnd(params)

		// @@@STEP_LOG_END_PERF@<label>@@@
	case "STEP_LOG_END_PERF":
		// Ignore for now. Ideally would emit a link to the perf dashboard.
		break

		// @@@STEP_CLEAR@@@
	case "STEP_CLEAR":
		step := s.CurrentStep()
		updatedIf(step, UpdateIterative, step.ClearText())

		// @@@STEP_SUMMARY_CLEAR@@@
	case "STEP_SUMMARY_CLEAR":
		step := s.CurrentStep()
		step.ClearSummary()
		updatedIf(step, UpdateIterative, true)

		// @@@STEP_TEXT@<msg>@@@
	case "STEP_TEXT":
		step := s.CurrentStep()
		updatedIf(step, UpdateIterative, step.AddText(params))

		// @@@SEED_STEP_TEXT@step@<msg>@@@
	case "SEED_STEP_TEXT":
		parts := strings.SplitN(params, "@", 2)
		if len(parts) < 2 {
			return nil
		}
		step, err := s.LookupStepErr(parts[0])
		if err != nil {
			return err
		}
		updatedIf(step, UpdateIterative, step.AddText(parts[1]))

		// @@@STEP_SUMMARY_TEXT@<msg>@@@
	case "STEP_SUMMARY_TEXT":
		step := s.CurrentStep()
		updatedIf(step, UpdateIterative, step.SetSummary(params))

		// @@@STEP_NEST_LEVEL@<level>@@@
	case "STEP_NEST_LEVEL":
		level, err := strconv.Atoi(params)
		if err != nil {
			return fmt.Errorf("could not parse nest level from %q: %v", params, err)
		}
		if level < 0 {
			return fmt.Errorf("level must be >= 0, not %d", level)
		}

		step := s.CurrentStep()
		updatedIf(step, UpdateStructural, step.SetNestLevel(level))
		break

		// @@@HALT_ON_FAILURE@@@
	case "HALT_ON_FAILURE":
		s.haltOnFailure = true

		// @@@HONOR_ZERO_RETURN_CODE@@@
	case "HONOR_ZERO_RETURN_CODE":
		// We don't capture the step return code, so not much we can do here.
		break

	// @@@SET_BUILD_PROPERTY@<name>@<json>@@@
	case "SET_BUILD_PROPERTY":
		step := s.CurrentStep()
		parts := strings.SplitN(params, "@", 2)
		if len(parts) == 1 {
			parts = append(parts, "")
		}
		updatedIf(step, UpdateIterative, step.SetProperty(parts[0], parts[1]))

		// @@@STEP_TRIGGER@<spec>@@@
	case "STEP_TRIGGER":
		// Annotee will stop short of sending an actual request to BuildBucket.
		break
	}

	if updated != nil {
		s.Callbacks.Updated(updated, updateType)
	}
	return nil
}

// Finish closes the top-level annotation state and any outstanding steps.
func (s *State) Finish() {
	s.initialize()
	s.finishAndDeriveStatus()
}

func (s *State) finishAndDeriveStatus() bool {
	return s.finishWithStatusImpl(nil, nil)
}

func (s *State) finishWithStatus(st milo.Status, fd *milo.FailureDetails) bool {
	return s.finishWithStatusImpl(&st, fd)
}

func (s *State) finishWithStatusImpl(status *milo.Status, fd *milo.FailureDetails) bool {
	if s.closed {
		return false
	}

	// if s.currentTimestamp is not nil, the last annotation was
	// CURRENT_TIMESTAMP and s.currentTimestamp contains its value.
	buildEndTime := s.currentTimestamp
	s.currentTimestamp = nil
	if buildEndTime == nil && !s.Offline {
		buildEndTime = s.now()
	}

	// Traverse through every step *except* our root step.
	unfinished := false
	for step := s.rootStep.nextStep; step != nil; step = step.nextStep {
		if u := step.closeWithStatus(buildEndTime, nil); u {
			unfinished = true
		}
	}

	// If some steps were unfinished, show a root exception.
	if unfinished && status == nil {
		exception := milo.Status_FAILURE
		status = &exception
		if fd == nil {
			fd = &milo.FailureDetails{
				Type: milo.FailureDetails_EXCEPTION,
			}
		}

	}
	s.rootStep.FailureDetails = fd
	s.rootStep.closeWithStatus(buildEndTime, status)

	// Probe the status from our steps, if one is not supplied.
	s.closed = true
	return true
}

// LookupStep returns the step with the supplied name, or nil if no such step
// exists.
//
// If multiple steps share a name, this will return the latest registered step
// with that name.
func (s *State) LookupStep(name string) *Step { return s.stepMap[name] }

// LookupStepErr returns the step with the supplied name, or an error if no
// such step exists.
//
// If multiple steps share a name, this will return the latest registered step
// with that name.
func (s *State) LookupStepErr(name string) (*Step, error) {
	if as := s.LookupStep(name); as != nil {
		return as, nil
	}
	return nil, fmt.Errorf("no step named %q", name)
}

// ResolveStep returns the annotation package *Step corresponding to the
// supplied *milo.Step. This is a reverse lookup operation.
//
// If the supplied *milo.Step is not registered with this annotation State,
// this function will return nil.
func (s *State) ResolveStep(ms *milo.Step) *Step { return s.stepLookup[ms] }

// RootStep returns the root step.
func (s *State) RootStep() *Step {
	s.initialize()

	return &s.rootStep
}

// AnnotationStream returns the name of this State's Milo annotation datagram
// stream.
func (s *State) AnnotationStream() types.StreamName {
	return s.rootStep.BaseStream("annotations")
}

// CurrentStep returns the step referenced by the step cursor.
func (s *State) CurrentStep() *Step {
	s.initialize()

	return s.stepCursor
}

// SetCurrentStep sets the current step. If the supplied step is nil, the root
// step will be used.
//
// The supplied step must already be registered with the State.
func (s *State) SetCurrentStep(v *Step) {
	if v == nil {
		v = &s.rootStep
	}
	if v.s != s {
		panic("step is not bound to state")
	}
	s.stepCursor = v
}

func (s *State) registerStep(as *Step) {
	s.stepMap[as.Name()] = as
	s.stepLookup[&as.Step] = as

	if latest := s.latestStep; latest != nil {
		latest.nextStep = as
		as.prevStep = latest
	}
	s.latestStep = as
}

func (s *State) unregisterStep(as *Step) {
	name := as.Name()
	if cas := s.stepMap[name]; cas == as {
		delete(s.stepMap, name)
	}

	if s.stepCursor == as {
		s.stepCursor = as.closestOpenStep()
	}
}

// now returns current time of s.Clock. Defaults to system clock.
func (s *State) now() *google.Timestamp {
	c := s.Clock
	if c == nil {
		c = clock.GetSystemClock()
	}
	return google.NewTimestamp(c.Now())
}

// Step represents a single step.
type Step struct {
	milo.Step
	s *State

	// parent is the step that spawned this step.
	parent *Step

	// prevStep is the step that was created immediately before this step. It is
	// nil if this is the root step.
	//
	// Both prevStep and nextStep are creation-ordered, and don't change even if
	// a Step is reparented.
	prevStep *Step
	// nextStep is the step that was created immediately after this step. It is
	// nil if this is the latest step.
	//
	// Both prevStep and nextStep are creation-ordered, and don't change even if
	// a Step is reparented.
	nextStep *Step

	index int
	level int

	stepIndex map[string]int

	// logLines is a map of log line labels to full log stream names.
	logLines map[string]types.StreamName
	// logLineCount is a map of log line label to the number of times that log
	// line has appeared. This is to prevent the case where multiple log lines
	// with the same label may be emitted, which would cause duplicate log stream
	// names.
	logLineCount map[string]int

	// LogNameBase is the LogDog stream name root for this step.
	logNameBase types.StreamName
	// hasSummary, if true, means that this Step has summary text. The summary
	// text is stored as the first line in its Step.Text slice.
	hasSummary bool
	// closed is true if the element is closed.
	closed bool
}

func (as *Step) initializeStep(s *State, parent *Step, name string, index int, logNameBase types.StreamName) *Step {
	t := milo.Status_RUNNING
	as.Step = milo.Step{
		Name:   name,
		Status: t,
	}

	as.s = s
	as.index = index
	as.logNameBase = logNameBase
	as.stepIndex = map[string]int{}
	as.logLines = map[string]types.StreamName{}
	as.logLineCount = map[string]int{}

	// Add this Step to our parent's Substep list.
	if parent != nil {
		parent.appendSubstep(as)
	}
	s.registerStep(as)

	return as
}

func (as *Step) appendSubstep(s *Step) {
	if s.parent == as {
		// Already parented to as, so do nothing.
		return
	}
	s.detachFromParent()

	s.parent = as
	as.Substep = append(as.Substep, &milo.Step_Substep{
		Substep: &milo.Step_Substep_Step{
			Step: &s.Step,
		},
	})
}

func (as *Step) detachFromParent() {
	parent := as.parent
	if parent == nil {
		return
	}

	// Remove any instances of "as" from its current parent's Substeps.
	ssPtr := 0
	for _, ss := range parent.Substep {
		if ss.GetStep() != &as.Step {
			parent.Substep[ssPtr] = ss
			ssPtr++
		}
	}
	parent.Substep = parent.Substep[:ssPtr]
	as.parent = nil
}

// CanonicalName returns the canonical name of this Step. This name is
// guaranteed to be unique witin the State.
func (as *Step) CanonicalName() string {
	parts := []string(nil)
	if as.index == 0 {
		parts = append(parts, as.Name())
	} else {
		parts = append(parts, fmt.Sprintf("%s_%d", as.Name(), as.index))
	}
	for p := as.parent; p != nil; p = p.parent {
		parts = append(parts, p.Name())
	}
	for i := len(parts)/2 - 1; i >= 0; i-- {
		opp := len(parts) - 1 - i
		parts[i], parts[opp] = parts[opp], parts[i]
	}
	return path.Join(parts...)
}

// Name returns the step's component name.
func (as *Step) Name() string {
	return as.Step.Name
}

// Proto returns the Milo protobuf associated with this Step.
func (as *Step) Proto() *milo.Step {
	return &as.Step
}

// BaseStream returns the supplied name prepended with this Step's base
// log name.
//
// For example, if the base name is "foo/bar", BaseStream("baz") will return
// "foo/bar/baz".
func (as *Step) BaseStream(name types.StreamName) types.StreamName {
	if as.logNameBase == "" {
		return name
	}
	return as.logNameBase.Concat(name)
}

// AddStep generates a new substep.
func (as *Step) AddStep(name string) *Step {
	// Determine/advance step index.
	index := as.stepIndex[name]
	as.stepIndex[name]++

	logPath, err := types.MakeStreamName("s_", "steps", name, strconv.Itoa(index))
	if err != nil {
		panic(fmt.Errorf("failed to generate step name for [%s]: %s", name, err))
	}

	nas := (&Step{}).initializeStep(as.s, as, name, index, as.BaseStream(logPath))
	return nas
}

// Start marks the Step as started.
func (as *Step) Start(startTime *google.Timestamp) bool {
	if as.Started != nil {
		return false
	}
	as.Started = startTime
	return true
}

// Close closes this step and any outstanding resources that it owns.
// If it is already closed, does not have side effects and returns false.
func (as *Step) Close(closeTime *google.Timestamp) bool {
	return as.closeWithStatus(closeTime, nil)
}

func (as *Step) closeWithStatus(closeTime *google.Timestamp, sp *milo.Status) bool {
	if as.closed {
		return false
	}

	// Close our outstanding substeps, and get their highest status value.
	stepStatus := milo.Status_SUCCESS
	if sp == nil {
		for _, ss := range as.Substep {
			sub := as.s.ResolveStep(ss.GetStep())
			if sub == nil {
				continue
			}

			sub.Close(closeTime)
			if sub.Status > stepStatus {
				stepStatus = sub.Status
			}
		}
	} else {
		// If a status is provided, use it.
		stepStatus = *sp
	}

	// Close any outstanding log streams.
	for l := range as.logLines {
		as.LogEnd(l)
	}

	if as.Status == milo.Status_RUNNING {
		as.Status = stepStatus
	}
	as.Ended = closeTime
	if as.Started == nil {
		as.Started = as.Ended
	}

	as.closed = true
	as.s.unregisterStep(as)
	as.s.Callbacks.Updated(as, UpdateStructural)
	as.s.Callbacks.StepClosed(as)
	return true
}

func (as *Step) closestOpenStep() *Step {
	for ps := as.prevStep; ps != nil; ps = ps.prevStep {
		if !ps.closed {
			return ps
		}
	}
	return &as.s.rootStep
}

// LogLine emits a log line for a specified log label.
func (as *Step) LogLine(label, line string) bool {
	updated := false

	name, ok := as.logLines[label]
	if !ok {
		// No entry for this log line. Create a new one and register it.
		//
		// This will appear as:
		// [BASE]/logs/[label]/[ord]
		subName, err := types.MakeStreamName("s_", "logs", label, strconv.Itoa(as.logLineCount[label]))
		if err != nil {
			panic(fmt.Errorf("failed to generate log stream name for [%s]: %s", label, err))
		}
		name = as.BaseStream(subName)
		as.AddLogdogStreamLink("", label, "", name)

		as.logLines[label] = name
		as.logLineCount[label]++
		updated = true
	}

	as.s.Callbacks.StepLogLine(as, name, label, line)
	return updated
}

// LogEnd ends the log for the specified label.
func (as *Step) LogEnd(label string) {
	name, ok := as.logLines[label]
	if !ok {
		return
	}

	delete(as.logLines, label)
	as.s.Callbacks.StepLogEnd(as, name)
}

// AddText adds a line of step component text.
func (as *Step) AddText(text string) bool {
	as.Text = append(as.Text, text)
	return true
}

// ClearText clears step component text.
func (as *Step) ClearText() bool {
	if len(as.Text) == 0 {
		return false
	}
	as.Text = nil
	return true
}

// SetSummary sets the Step's summary text.
//
// The summary is implemented as the first line of step component text. If no
// summary is currently defined, one will be inserted; otherwise, the current
// summary will be replaced.
func (as *Step) SetSummary(value string) bool {
	if as.hasSummary {
		if as.Text[0] == value {
			return false
		}

		as.Text[0] = value
	} else {
		as.Text = append(as.Text, "")
		copy(as.Text[1:], as.Text)
		as.Text[0] = value
		as.hasSummary = true
	}
	return true
}

// ClearSummary clears the step's summary text.
func (as *Step) ClearSummary() {
	if as.hasSummary {
		as.Text = as.Text[:copy(as.Text, as.Text[1:])]
		as.hasSummary = false
	}
}

// SetNestLevel sets the nest level of this Step, and identifies its nesting
// parent.
//
// If no parent could be found at level "l-1", the root step will become the
// parent.
func (as *Step) SetNestLevel(l int) bool {
	if as.level == l {
		return false
	}
	as.level = l

	// Attach this step to the correct parent step based on nest level. Ascend
	// up the previously-declared steps.
	var nestParent *Step
	for prev := as.prevStep; prev != nil; prev = prev.prevStep {
		if prev.level < l {
			nestParent = prev
			break
		}
	}
	if nestParent == nil || nestParent == as.parent {
		return true
	}
	nestParent.appendSubstep(as)
	return true
}

// AddLogdogStreamLink adds a LogDog stream link to this Step's links list.
func (as *Step) AddLogdogStreamLink(server, label string, prefix, name types.StreamName) {
	link := &milo.Link{
		Label: label,
		Value: &milo.Link_LogdogStream{&milo.LogdogStream{
			Name:   string(name),
			Server: server,
			Prefix: string(prefix),
		}},
	}
	as.OtherLinks = append(as.OtherLinks, link)
}

// AddURLLink adds a URL link to this Step's links list.
func (as *Step) AddURLLink(label, alias, url string) {
	link := &milo.Link{
		Label:      label,
		AliasLabel: alias,
		Value:      &milo.Link_Url{url},
	}
	as.OtherLinks = append(as.OtherLinks, link)
}

// SetStatus sets this step's component status.
//
// If the status doesn't change, the supplied failure details will be ignored.
func (as *Step) SetStatus(s milo.Status, fd *milo.FailureDetails) bool {
	if as.closed || as.Status == s {
		return false
	}
	as.Status = s
	as.FailureDetails = fd
	return true
}

// SetProperty sets a key/value property for this Step.
func (as *Step) SetProperty(name, value string) bool {
	for _, p := range as.Property {
		if p.Name == name {
			if p.Value == value {
				return false
			}
			p.Value = value
			return true
		}
	}

	as.Property = append(as.Property, &milo.Step_Property{
		Name:  name,
		Value: value,
	})
	return true
}

// SetSTDOUTStream sets the LogDog STDOUT stream value, returning true if the
// Step was updated.
func (as *Step) SetSTDOUTStream(st *milo.LogdogStream) (updated bool) {
	as.StdoutStream, updated = as.maybeSetLogDogStream(as.StdoutStream, st)
	return
}

// SetSTDERRStream sets the LogDog STDERR stream value, returning true if the
// Step was updated.
func (as *Step) SetSTDERRStream(st *milo.LogdogStream) (updated bool) {
	as.StderrStream, updated = as.maybeSetLogDogStream(as.StderrStream, st)
	return
}

func (as *Step) maybeSetLogDogStream(target *milo.LogdogStream, st *milo.LogdogStream) (*milo.LogdogStream, bool) {
	if (target == nil && st == nil) || (target != nil && st != nil && *target == *st) {
		return target, false
	}
	return st, true
}
