package types

import (
	"bytes"
	"encoding/gob"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// TaskComment contains a comment about a Task. {Repo, Revision, Name,
// Timestamp} is used as the unique id for this comment. If TaskId is empty, the
// comment applies to all matching tasks.
type TaskComment struct {
	Repo     string `json:"repo"`
	Revision string `json:"revision"`
	Name     string `json:"name"` // Name of TaskSpec.
	// Timestamp is compared ignoring timezone. The timezone reflects User's
	// location.
	Timestamp time.Time `json:"time"`
	TaskId    string    `json:"taskId,omitempty"`
	User      string    `json:"user,omitempty"`
	Message   string    `json:"message,omitempty"`
	Deleted   *bool     `json:"deleted,omitempty"`
}

func (c TaskComment) Copy() *TaskComment {
	rv := &c
	if c.Deleted != nil {
		v := *c.Deleted
		rv.Deleted = &v
	}
	return rv
}

func (c *TaskComment) Id() string {
	return c.Repo + "#" + c.Revision + "#" + c.Name + "#" + c.Timestamp.Format(util.SAFE_TIMESTAMP_FORMAT)
}

// TaskSpecComment contains a comment about a TaskSpec. {Repo, Name, Timestamp}
// is used as the unique id for this comment.
type TaskSpecComment struct {
	Repo string `json:"repo"`
	Name string `json:"name"` // Name of TaskSpec.
	// Timestamp is compared ignoring timezone. The timezone reflects User's
	// location.
	Timestamp     time.Time `json:"time"`
	User          string    `json:"user,omitempty"`
	Flaky         bool      `json:"flaky"`
	IgnoreFailure bool      `json:"ignoreFailure"`
	Message       string    `json:"message,omitempty"`
	Deleted       *bool     `json:"deleted,omitempty"`
}

func (c TaskSpecComment) Copy() *TaskSpecComment {
	rv := &c
	if c.Deleted != nil {
		v := *c.Deleted
		rv.Deleted = &v
	}
	return rv
}

func (c *TaskSpecComment) Id() string {
	return c.Repo + "#" + c.Name + "#" + c.Timestamp.Format(util.SAFE_TIMESTAMP_FORMAT)
}

// CommitComment contains a comment about a commit. {Repo, Revision, Timestamp}
// is used as the unique id for this comment.
type CommitComment struct {
	Repo     string `json:"repo"`
	Revision string `json:"revision"`
	// Timestamp is compared ignoring timezone. The timezone reflects User's
	// location.
	Timestamp     time.Time `json:"time"`
	User          string    `json:"user,omitempty"`
	IgnoreFailure bool      `json:"ignoreFailure"`
	Message       string    `json:"message,omitempty"`
	Deleted       *bool     `json:"deleted,omitempty"`
}

func (c CommitComment) Copy() *CommitComment {
	rv := &c
	if c.Deleted != nil {
		v := *c.Deleted
		rv.Deleted = &v
	}
	return rv
}

func (c *CommitComment) Id() string {
	return c.Repo + "#" + c.Revision + "#" + c.Timestamp.Format(util.SAFE_TIMESTAMP_FORMAT)
}

// RepoComments contains comments that all pertain to the same repository.
type RepoComments struct {
	// Repo is the repository (Repo field) of all the comments contained in
	// this RepoComments.
	Repo string
	// TaskComments maps commit hash and TaskSpec name to the comments for
	// the matching Task, sorted by timestamp.
	TaskComments map[string]map[string][]*TaskComment
	// TaskSpecComments maps TaskSpec name to the comments for that
	// TaskSpec, sorted by timestamp.
	TaskSpecComments map[string][]*TaskSpecComment
	// CommitComments maps commit hash to the comments for that commit,
	// sorted by timestamp.
	CommitComments map[string][]*CommitComment
}

func (orig *RepoComments) Copy() *RepoComments {
	// TODO(benjaminwagner): Make this more efficient.
	b := bytes.Buffer{}
	if err := gob.NewEncoder(&b).Encode(orig); err != nil {
		sklog.Fatal(err)
	}
	copy := RepoComments{}
	if err := gob.NewDecoder(&b).Decode(&copy); err != nil {
		sklog.Fatal(err)
	}
	return &copy
}

// TaskCommentSlice implements sort.Interface. To sort taskComments
// []*TaskComment, use sort.Sort(TaskCommentSlice(taskComments)).
type TaskCommentSlice []*TaskComment

func (s TaskCommentSlice) Len() int { return len(s) }

func (s TaskCommentSlice) Less(i, j int) bool {
	return s[i].Timestamp.Before(s[j].Timestamp)
}

func (s TaskCommentSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// TaskSpecCommentSlice implements sort.Interface. To sort taskSpecComments
// []*TaskSpecComment, use sort.Sort(TaskSpecCommentSlice(taskSpecComments)).
type TaskSpecCommentSlice []*TaskSpecComment

func (s TaskSpecCommentSlice) Len() int { return len(s) }

func (s TaskSpecCommentSlice) Less(i, j int) bool {
	return s[i].Timestamp.Before(s[j].Timestamp)
}

func (s TaskSpecCommentSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// CommitCommentSlice implements sort.Interface. To sort commitComments
// []*CommitComment, use sort.Sort(CommitCommentSlice(commitComments)).
type CommitCommentSlice []*CommitComment

func (s CommitCommentSlice) Len() int { return len(s) }

func (s CommitCommentSlice) Less(i, j int) bool {
	return s[i].Timestamp.Before(s[j].Timestamp)
}

func (s CommitCommentSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// TaskCommentEncoder encodes Tasks into bytes via GOB encoding. Not safe for
// concurrent use.
// TODO(benjaminwagner): Encode in parallel.
type TaskCommentEncoder struct {
	err      error
	comments []*TaskComment
	result   [][]byte
}

// Process encodes the Task into a byte slice that will be returned from Next()
// (in arbitrary order). Returns false if Next is certain to return an error.
// Caller must ensure t does not change until after the first call to Next().
// May not be called after calling Next().
func (e *TaskCommentEncoder) Process(c *TaskComment) bool {
	if e.err != nil {
		return false
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(c); err != nil {
		e.err = err
		e.comments = nil
		e.result = nil
		return false
	}
	e.comments = append(e.comments, c)
	e.result = append(e.result, buf.Bytes())
	return true
}

// Next returns one of the Tasks provided to Process (in arbitrary order) and
// its serialized bytes. If any comments remain, returns the task, the serialized
// bytes, nil. If all comments have been returned, returns nil, nil, nil. If an
// error is encountered, returns nil, nil, error.
func (e *TaskCommentEncoder) Next() (*TaskComment, []byte, error) {
	if e.err != nil {
		return nil, nil, e.err
	}
	if len(e.comments) == 0 {
		return nil, nil, nil
	}
	c := e.comments[0]
	e.comments = e.comments[1:]
	serialized := e.result[0]
	e.result = e.result[1:]
	return c, serialized, nil
}

// TaskCommentDecoder decodes bytes into TaskComments via GOB decoding. Not safe
// for concurrent use.
type TaskCommentDecoder struct {
	// input contains the incoming byte slices. Process() sends on this channel,
	// decode() receives from it, and Result() closes it.
	input chan []byte
	// output contains decoded TaskComments. decode() sends on this channel,
	// collect() receives from it, and run() closes it when all decode()
	// goroutines have finished.
	output chan *TaskComment
	// result contains the return value of Result(). collect() sends a single
	// value on this channel and closes it. Result() receives from it.
	result chan []*TaskComment
	// errors contains the first error from any goroutine. It's a channel in case
	// multiple goroutines experience an error at the same time.
	errors chan error
}

// init initializes d if it has not been initialized. May not be called concurrently.
func (d *TaskCommentDecoder) init() {
	if d.input == nil {
		d.input = make(chan []byte, kNumDecoderGoroutines*2)
		d.output = make(chan *TaskComment, kNumDecoderGoroutines)
		d.result = make(chan []*TaskComment, 1)
		d.errors = make(chan error, kNumDecoderGoroutines)
		go d.run()
		go d.collect()
	}
}

// run starts the decode goroutines and closes d.output when they finish.
func (d *TaskCommentDecoder) run() {
	// Start decoders.
	wg := sync.WaitGroup{}
	for i := 0; i < kNumDecoderGoroutines; i++ {
		wg.Add(1)
		go d.decode(&wg)
	}
	// Wait for decoders to exit.
	wg.Wait()
	// Drain d.input in the case that errors were encountered, to avoid deadlock.
	for range d.input {
	}
	close(d.output)
}

// decode receives from d.input and sends to d.output until d.input is closed or
// d.errors is non-empty. Decrements wg when done.
func (d *TaskCommentDecoder) decode(wg *sync.WaitGroup) {
	for b := range d.input {
		var c TaskComment
		if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&c); err != nil {
			d.errors <- err
			break
		}
		d.output <- &c
		if len(d.errors) > 0 {
			break
		}
	}
	wg.Done()
}

// collect receives from d.output until it is closed, then sends on d.result.
func (d *TaskCommentDecoder) collect() {
	result := []*TaskComment{}
	for c := range d.output {
		result = append(result, c)
	}
	d.result <- result
	close(d.result)
}

// Process decodes the byte slice into a TaskComment and includes it in Result()
// (in arbitrary order). Returns false if Result is certain to return an error.
// Caller must ensure b does not change until after Result() returns.
func (d *TaskCommentDecoder) Process(b []byte) bool {
	d.init()
	d.input <- b
	return len(d.errors) == 0
}

// Result returns all decoded TaskComments provided to Process (in arbitrary
// order), or any error encountered.
func (d *TaskCommentDecoder) Result() ([]*TaskComment, error) {
	// Allow TaskCommentDecoder to be used without initialization.
	if d.result == nil {
		return []*TaskComment{}, nil
	}
	close(d.input)
	select {
	case err := <-d.errors:
		return nil, err
	case result := <-d.result:
		return result, nil
	}
}

// TaskSpecCommentEncoder encodes Tasks into bytes via GOB encoding. Not safe for
// concurrent use.
// TODO(benjaminwagner): Encode in parallel.
type TaskSpecCommentEncoder struct {
	err      error
	comments []*TaskSpecComment
	result   [][]byte
}

// Process encodes the Task into a byte slice that will be returned from Next()
// (in arbitrary order). Returns false if Next is certain to return an error.
// Caller must ensure t does not change until after the first call to Next().
// May not be called after calling Next().
func (e *TaskSpecCommentEncoder) Process(c *TaskSpecComment) bool {
	if e.err != nil {
		return false
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(c); err != nil {
		e.err = err
		e.comments = nil
		e.result = nil
		return false
	}
	e.comments = append(e.comments, c)
	e.result = append(e.result, buf.Bytes())
	return true
}

// Next returns one of the Tasks provided to Process (in arbitrary order) and
// its serialized bytes. If any comments remain, returns the task, the serialized
// bytes, nil. If all comments have been returned, returns nil, nil, nil. If an
// error is encountered, returns nil, nil, error.
func (e *TaskSpecCommentEncoder) Next() (*TaskSpecComment, []byte, error) {
	if e.err != nil {
		return nil, nil, e.err
	}
	if len(e.comments) == 0 {
		return nil, nil, nil
	}
	c := e.comments[0]
	e.comments = e.comments[1:]
	serialized := e.result[0]
	e.result = e.result[1:]
	return c, serialized, nil
}

// TaskSpecCommentDecoder decodes bytes into Tasks via GOB decoding. Not safe for
// concurrent use.
type TaskSpecCommentDecoder struct {
	// input contains the incoming byte slices. Process() sends on this channel,
	// decode() receives from it, and Result() closes it.
	input chan []byte
	// output contains decoded Tasks. decode() sends on this channel, collect()
	// receives from it, and run() closes it when all decode() goroutines have
	// finished.
	output chan *TaskSpecComment
	// result contains the return value of Result(). collect() sends a single
	// value on this channel and closes it. Result() receives from it.
	result chan []*TaskSpecComment
	// errors contains the first error from any goroutine. It's a channel in case
	// multiple goroutines experience an error at the same time.
	errors chan error
}

// init initializes d if it has not been initialized. May not be called concurrently.
func (d *TaskSpecCommentDecoder) init() {
	if d.input == nil {
		d.input = make(chan []byte, kNumDecoderGoroutines*2)
		d.output = make(chan *TaskSpecComment, kNumDecoderGoroutines)
		d.result = make(chan []*TaskSpecComment, 1)
		d.errors = make(chan error, kNumDecoderGoroutines)
		go d.run()
		go d.collect()
	}
}

// run starts the decode goroutines and closes d.output when they finish.
func (d *TaskSpecCommentDecoder) run() {
	// Start decoders.
	wg := sync.WaitGroup{}
	for i := 0; i < kNumDecoderGoroutines; i++ {
		wg.Add(1)
		go d.decode(&wg)
	}
	// Wait for decoders to exit.
	wg.Wait()
	// Drain d.input in the case that errors were encountered, to avoid deadlock.
	for range d.input {
	}
	close(d.output)
}

// decode receives from d.input and sends to d.output until d.input is closed or
// d.errors is non-empty. Decrements wg when done.
func (d *TaskSpecCommentDecoder) decode(wg *sync.WaitGroup) {
	for b := range d.input {
		var c TaskSpecComment
		if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&c); err != nil {
			d.errors <- err
			break
		}
		d.output <- &c
		if len(d.errors) > 0 {
			break
		}
	}
	wg.Done()
}

// collect receives from d.output until it is closed, then sends on d.result.
func (d *TaskSpecCommentDecoder) collect() {
	result := []*TaskSpecComment{}
	for c := range d.output {
		result = append(result, c)
	}
	d.result <- result
	close(d.result)
}

// Process decodes the byte slice into a TaskSpecComment and includes it in
// Result() (in arbitrary order). Returns false if Result is certain to return
// an error. Caller must ensure b does not change until after Result() returns.
func (d *TaskSpecCommentDecoder) Process(b []byte) bool {
	d.init()
	d.input <- b
	return len(d.errors) == 0
}

// Result returns all decoded TaskSpecComments provided to Process (in arbitrary
// order), or any error encountered.
func (d *TaskSpecCommentDecoder) Result() ([]*TaskSpecComment, error) {
	// Allow TaskSpecCommentDecoder to be used without initialization.
	if d.result == nil {
		return []*TaskSpecComment{}, nil
	}
	close(d.input)
	select {
	case err := <-d.errors:
		return nil, err
	case result := <-d.result:
		return result, nil
	}
}

// CommitCommentEncoder encodes CommitComments into bytes via GOB encoding. Not
// safe for concurrent use.
// TODO(benjaminwagner): Encode in parallel.
type CommitCommentEncoder struct {
	err      error
	comments []*CommitComment
	result   [][]byte
}

// Process encodes the CommitComment into a byte slice that will be returned from Next()
// (in arbitrary order). Returns false if Next is certain to return an error.
// Caller must ensure t does not change until after the first call to Next().
// May not be called after calling Next().
func (e *CommitCommentEncoder) Process(c *CommitComment) bool {
	if e.err != nil {
		return false
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(c); err != nil {
		e.err = err
		e.comments = nil
		e.result = nil
		return false
	}
	e.comments = append(e.comments, c)
	e.result = append(e.result, buf.Bytes())
	return true
}

// Next returns one of the CommitComments provided to Process (in arbitrary
// order) and its serialized bytes. If any comments remain, returns the comment,
// the serialized bytes, nil. If all comments have been returned, returns nil,
// nil, nil. If an error is encountered, returns nil, nil, error.
func (e *CommitCommentEncoder) Next() (*CommitComment, []byte, error) {
	if e.err != nil {
		return nil, nil, e.err
	}
	if len(e.comments) == 0 {
		return nil, nil, nil
	}
	c := e.comments[0]
	e.comments = e.comments[1:]
	serialized := e.result[0]
	e.result = e.result[1:]
	return c, serialized, nil
}

// CommitCommentDecoder decodes bytes into CommitComments via GOB decoding.
// Not safe for concurrent use.
type CommitCommentDecoder struct {
	// input contains the incoming byte slices. Process() sends on this channel,
	// decode() receives from it, and Result() closes it.
	input chan []byte
	// output contains decoded Tasks. decode() sends on this channel, collect()
	// receives from it, and run() closes it when all decode() goroutines have
	// finished.
	output chan *CommitComment
	// result contains the return value of Result(). collect() sends a single
	// value on this channel and closes it. Result() receives from it.
	result chan []*CommitComment
	// errors contains the first error from any goroutine. It's a channel in case
	// multiple goroutines experience an error at the same time.
	errors chan error
}

// init initializes d if it has not been initialized. May not be called concurrently.
func (d *CommitCommentDecoder) init() {
	if d.input == nil {
		d.input = make(chan []byte, kNumDecoderGoroutines*2)
		d.output = make(chan *CommitComment, kNumDecoderGoroutines)
		d.result = make(chan []*CommitComment, 1)
		d.errors = make(chan error, kNumDecoderGoroutines)
		go d.run()
		go d.collect()
	}
}

// run starts the decode goroutines and closes d.output when they finish.
func (d *CommitCommentDecoder) run() {
	// Start decoders.
	wg := sync.WaitGroup{}
	for i := 0; i < kNumDecoderGoroutines; i++ {
		wg.Add(1)
		go d.decode(&wg)
	}
	// Wait for decoders to exit.
	wg.Wait()
	// Drain d.input in the case that errors were encountered, to avoid deadlock.
	for range d.input {
	}
	close(d.output)
}

// decode receives from d.input and sends to d.output until d.input is closed or
// d.errors is non-empty. Decrements wg when done.
func (d *CommitCommentDecoder) decode(wg *sync.WaitGroup) {
	for b := range d.input {
		var c CommitComment
		if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&c); err != nil {
			d.errors <- err
			break
		}
		d.output <- &c
		if len(d.errors) > 0 {
			break
		}
	}
	wg.Done()
}

// collect receives from d.output until it is closed, then sends on d.result.
func (d *CommitCommentDecoder) collect() {
	result := []*CommitComment{}
	for c := range d.output {
		result = append(result, c)
	}
	d.result <- result
	close(d.result)
}

// Process decodes the byte slice into a CommitComment and includes it in
// Result() (in arbitrary order). Returns false if Result is certain to return
// an error. Caller must ensure b does not change until after Result() returns.
func (d *CommitCommentDecoder) Process(b []byte) bool {
	d.init()
	d.input <- b
	return len(d.errors) == 0
}

// Result returns all decoded CommitComments provided to Process (in arbitrary
// order), or any error encountered.
func (d *CommitCommentDecoder) Result() ([]*CommitComment, error) {
	// Allow CommitCommentDecoder to be used without initialization.
	if d.result == nil {
		return []*CommitComment{}, nil
	}
	close(d.input)
	select {
	case err := <-d.errors:
		return nil, err
	case result := <-d.result:
		return result, nil
	}
}
