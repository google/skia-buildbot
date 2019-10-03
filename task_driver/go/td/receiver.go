package td

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"

	"cloud.google.com/go/logging"
	"github.com/golang/glog"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Receiver is an interface used to implement arbitrary receivers of step
// metadata, as steps are run.
type Receiver interface {
	// Handle the given message.
	HandleMessage(*Message) error
	LogStream(stepId string, logId string, severity string) (io.Writer, error)
	Close() error
}

// MultiReceiver is a Receiver which multiplexes messages to multiple Receivers.
type MultiReceiver []Receiver

// See documentation for Receiver interface.
func (r MultiReceiver) HandleMessage(m *Message) error {
	g := util.NewNamedErrGroup()
	for _, rec := range r {
		receiver := rec
		name := fmt.Sprint(reflect.TypeOf(receiver))
		g.Go(name, func() error {
			return receiver.HandleMessage(m)
		})
	}
	return g.Wait()
}

// See documentation for Receiver interface.
func (r MultiReceiver) LogStream(stepId, logId, severity string) (io.Writer, error) {
	writers := make([]io.Writer, 0, len(r))
	for _, rec := range r {
		w, err := rec.LogStream(stepId, logId, severity)
		if err != nil {
			return nil, err
		}
		writers = append(writers, w)
	}
	return util.MultiWriter(writers), nil
}

// See documentation for Receiver interface.
func (r MultiReceiver) Close() error {
	g := util.NewNamedErrGroup()
	for _, rec := range r {
		receiver := rec
		name := fmt.Sprint(reflect.TypeOf(receiver))
		g.Go(name, func() error {
			return receiver.Close()
		})
	}
	return g.Wait()
}

// DebugReceiver just dumps the messages straight to the log (stdout/stderr, not
// to Cloud Logging).
type DebugReceiver struct{}

// See documentation for Receiver interface.
func (r *DebugReceiver) HandleMessage(m *Message) error {
	switch m.Type {
	case MSG_TYPE_RUN_STARTED:
		glog.Infof("RUN_STARTED: %+v", m.Run)
	case MSG_TYPE_STEP_STARTED:
		glog.Infof("STEP_STARTED: %s", m.StepId)
	case MSG_TYPE_STEP_FINISHED:
		glog.Infof("STEP_FINISHED: %s", m.StepId)
	case MSG_TYPE_STEP_EXCEPTION:
		glog.Infof("STEP_EXCEPTION: %s", m.StepId)
	case MSG_TYPE_STEP_FAILED:
		glog.Infof("STEP_FAILED: %s", m.StepId)
	case MSG_TYPE_STEP_DATA:
		b, err := json.MarshalIndent(m.Data, "", " ")
		if err != nil {
			return err
		}
		glog.Infof("STEP_DATA: %s: %s", m.StepId, string(b))
	default:
		return fmt.Errorf("Invalid message type %s", m.Type)
	}
	return nil
}

// See documentation for Receiver interface.
func (r *DebugReceiver) LogStream(stepId, logId, severity string) (io.Writer, error) {
	sev := logging.ParseSeverity(severity)
	if sev >= logging.Warning {
		return os.Stderr, nil
	}
	return os.Stdout, nil
}

// See documentation for Receiver interface.
func (r *DebugReceiver) Close() error {
	glog.Info("Run finished.")
	return nil
}

// StepReport is a struct used to collect information about a given step.
type StepReport struct {
	*StepProperties
	Data       []interface{} `json:"data,omitempty"`
	Errors     []string      `json:"errors,omitempty"`
	Exceptions []string      `json:"exceptions,omitempty"`
	logs       map[string]*bytes.Buffer
	Result     StepResult    `json:"result,omitempty"`
	Steps      []*StepReport `json:"steps,omitempty"`
}

// ReportReceiver collects all messages and generates a report when requested.
type ReportReceiver struct {
	mtx    sync.Mutex
	root   *StepReport
	output string
}

// newReportReceiver returns a ReportReceiver instance.
func newReportReceiver(output string) *ReportReceiver {
	return &ReportReceiver{
		output: output,
	}
}

// Recurse through all steps, running the given function. If the function
// returns false, recursion stops.
func (s *StepReport) Recurse(fn func(*StepReport) bool) bool {
	if keepGoing := fn(s); !keepGoing {
		return false
	}
	for _, sub := range s.Steps {
		if keepGoing := sub.Recurse(fn); !keepGoing {
			return false
		}
	}
	return true
}

// Find the step with the given ID in our list. This helps in case messages
// arrive out of order.
func (s *StepReport) findStep(id string) (*StepReport, error) {
	var rv *StepReport
	s.Recurse(func(s *StepReport) bool {
		if s.Id == id {
			rv = s
			return false
		}
		return true
	})
	if rv != nil {
		return rv, nil
	}
	return nil, fmt.Errorf("Unknown step ID %q", id)
}

// Find the step with the given ID in our list. This helps in case messages
// arrive out of order.
func (r *ReportReceiver) findStep(id string) (*StepReport, error) {
	if r.root == nil {
		return nil, fmt.Errorf("No steps!")
	}
	return r.root.findStep(id)
}

// See documentation for Receiver interface.
func (r *ReportReceiver) HandleMessage(m *Message) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	switch m.Type {
	case MSG_TYPE_RUN_STARTED:
		// Do nothing.
	case MSG_TYPE_STEP_STARTED:
		s := &StepReport{
			StepProperties: m.Step,
			logs:           map[string]*bytes.Buffer{},
		}
		if m.Step.Id == STEP_ID_ROOT {
			r.root = s
		} else {
			parent, err := r.findStep(m.Step.Parent)
			if err != nil {
				return err
			}
			parent.Steps = append(parent.Steps, s)
		}
	case MSG_TYPE_STEP_FINISHED:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		if len(s.Errors) == 0 && len(s.Exceptions) == 0 {
			s.Result = STEP_RESULT_SUCCESS
		}
	case MSG_TYPE_STEP_FAILED:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		s.Errors = append(s.Errors, m.Error)
		s.Result = STEP_RESULT_FAILURE
	case MSG_TYPE_STEP_EXCEPTION:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		s.Exceptions = append(s.Exceptions, m.Error)
		s.Result = STEP_RESULT_EXCEPTION
	case MSG_TYPE_STEP_DATA:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		s.Data = append(s.Data, m.Data)
	}
	return nil
}

// See documentation for Receiver interface.
func (r *ReportReceiver) Close() error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	if r.output == "" {
		return nil
	}

	// Visit each step, attaching the final log contents to each logData
	// instance.
	r.root.Recurse(func(s *StepReport) bool {
		for _, data := range s.Data {
			d, ok := data.(*LogData)
			if ok {
				if logBuf, ok := s.logs[d.Id]; ok {
					d.Log = logBuf.String()
				}
			}
		}
		return true
	})

	// Dump JSON to the given Writer.
	b, err := json.MarshalIndent(r.root, "", "  ")
	if err != nil {
		return err
	}
	// Write the report to the desired output.
	if r.output == "-" {
		_, err = os.Stdout.Write(b)
		return err
	}
	return util.WithWriteFile(r.output, func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	})
}

// See documentation for Receiver interface.
func (r *ReportReceiver) LogStream(stepId, logId, severity string) (io.Writer, error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	buf := bytes.NewBuffer([]byte{})
	step, err := r.findStep(stepId)
	if err != nil {
		return nil, err
	}
	if _, ok := step.logs[logId]; ok {
		return nil, fmt.Errorf("Step %s already has a log with ID %s", stepId, logId)
	}
	step.logs[logId] = buf
	return buf, nil
}

// CloudLoggingReceiver is a Receiver which sends step metadata and logs to
// Cloud Logging.
type CloudLoggingReceiver struct {
	// logger is a handle to the Logger used for the entire test run.
	logger *logging.Logger
}

// Return a new CloudLoggingReceiver. This initializes Cloud Logging for the
// entire test run.
func NewCloudLoggingReceiver(logger *logging.Logger) (*CloudLoggingReceiver, error) {
	return &CloudLoggingReceiver{
		logger: logger,
	}, nil
}

// See documentation for Receiver interface.
func (r *CloudLoggingReceiver) HandleMessage(m *Message) error {
	// TODO(borenet): When should we LogSync, or Flush? If the program
	// crashes or is killed, we'll want to have already flushed the logs.
	labels := map[string]string{}
	if m.StepId != "" {
		labels["stepId"] = m.StepId
	}
	r.logger.Log(logging.Entry{
		Payload:  m,
		Severity: logging.ParseSeverity(sklog.DEBUG),
		Labels:   labels,
	})
	return nil
}

// cloudLogsWriter is an io.Writer which writes to Cloud Logging.
type cloudLogsWriter struct {
	logger   *logging.Logger
	labels   map[string]string
	severity logging.Severity
}

// See documentation for io.Writer.
func (w *cloudLogsWriter) Write(b []byte) (int, error) {
	// TODO(borenet): Should we buffer until we see a newline?
	// TODO(borenet): When should we LogSync, or Flush? If the program
	// crashes or is killed, we'll want to have already flushed the logs.
	w.logger.Log(logging.Entry{
		Labels:   w.labels,
		Payload:  string(b),
		Severity: w.severity,
	})
	return len(b), nil
}

// See documentation for Receiver interface.
func (r *CloudLoggingReceiver) LogStream(stepId, logId, severity string) (io.Writer, error) {
	return &cloudLogsWriter{
		logger: r.logger,
		labels: map[string]string{
			"logId":  logId,
			"stepId": stepId,
		},
		severity: logging.ParseSeverity(severity),
	}, nil
}

// See documentation for Receiver interface.
func (r *CloudLoggingReceiver) Close() error {
	return r.logger.Flush()
}
