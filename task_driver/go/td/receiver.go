package td

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"

	"cloud.google.com/go/logging"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Severity indicates the importance of a LogStream, with greater values
// indicating greater severity. Valid values include Debug, Info, Warning, and
// Error.
type Severity int

const (
	// SeverityDebug is the lowest, most verbose log severity, containing
	// messages only needed for detailed debugging.
	SeverityDebug = Severity(logging.Debug)
	// SeverityInfo is the base severity level for normal, informational
	// messages.
	SeverityInfo = Severity(logging.Info)
	// SeverityWarning indicates higher-priority messages which may indicate a
	// problem of some kind.
	SeverityWarning = Severity(logging.Warning)
	// SeverityError indicates high-priority messages which report actual
	// errors.
	SeverityError = Severity(logging.Error)
)

// asCloudLoggingSeverity returns the equivalent logging.Severity for s.
func (s Severity) asCloudLoggingSeverity() logging.Severity {
	return logging.Severity(s)
}

// String returns the name of s.
func (s Severity) String() string {
	return s.asCloudLoggingSeverity().String()
}

// Receiver is an interface used to implement arbitrary receivers of step
// metadata, as steps are run.
type Receiver interface {
	// Handle the given message.
	HandleMessage(*Message) error
	LogStream(stepId string, logId string, severity Severity) (io.Writer, error)
	Close() error
}

// MultiReceiver is a Receiver which multiplexes messages to multiple Receivers.
type MultiReceiver []Receiver

// HandleMessage implements Receiver.
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

// LogStream implements Receiver.
func (r MultiReceiver) LogStream(stepId, logId string, severity Severity) (io.Writer, error) {
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

// Close implements Receiver.
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

// HandleMessage implements Receiver.
func (r *DebugReceiver) HandleMessage(m *Message) error {
	switch m.Type {
	case MsgType_RunStarted:
		sklog.Infof("RUN_STARTED: %+v", m.Run)
	case MsgType_StepStarted:
		sklog.Infof("STEP_STARTED: %s", m.StepId)
	case MsgType_StepFinished:
		sklog.Infof("STEP_FINISHED: %s", m.StepId)
	case MsgType_StepException:
		sklog.Infof("STEP_EXCEPTION: %s", m.StepId)
	case MsgType_StepFailed:
		sklog.Infof("STEP_FAILED: %s", m.StepId)
	case MsgType_StepData:
		b, err := json.MarshalIndent(m.Data, "", " ")
		if err != nil {
			return err
		}
		sklog.Infof("STEP_DATA: %s: %s", m.StepId, string(b))
	default:
		return fmt.Errorf("Invalid message type %s", m.Type)
	}
	return nil
}

// LogStream implements Receiver.
func (r *DebugReceiver) LogStream(stepId, logId string, severity Severity) (io.Writer, error) {
	if severity >= SeverityWarning {
		return os.Stderr, nil
	}
	return os.Stdout, nil
}

// Close implements Receiver.
func (r *DebugReceiver) Close() error {
	sklog.Info("Run finished.")
	return nil
}

// StepReport is a struct used to collect information about a given step.
type StepReport struct {
	*StepProperties
	Data       []interface{} `json:"data,omitempty"`
	Errors     []string      `json:"errors,omitempty"`
	Exceptions []string      `json:"exceptions,omitempty"`
	Logs       map[string]*bytes.Buffer
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

// HandleMessage implements Receiver.
func (r *ReportReceiver) HandleMessage(m *Message) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	switch m.Type {
	case MsgType_RunStarted:
		// Do nothing.
	case MsgType_StepStarted:
		s := &StepReport{
			StepProperties: m.Step,
			Logs:           map[string]*bytes.Buffer{},
		}
		if strings.Contains(m.Step.Id, StepIDRoot) {
			r.root = s
		} else {
			parent, err := r.findStep(m.Step.Parent)
			if err != nil {
				return err
			}
			parent.Steps = append(parent.Steps, s)
		}
	case MsgType_StepFinished:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		if len(s.Errors) == 0 && len(s.Exceptions) == 0 {
			s.Result = StepResultSuccess
		}
	case MsgType_StepFailed:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		s.Errors = append(s.Errors, m.Error)
		s.Result = StepResultFailure
	case MsgType_StepException:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		s.Exceptions = append(s.Exceptions, m.Error)
		s.Result = StepResultException
	case MsgType_StepData:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		s.Data = append(s.Data, m.Data)
	}
	return nil
}

// Close implements Receiver.
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
				if logBuf, ok := s.Logs[d.Id]; ok {
					d.Log = logBuf.String()
				}
			}
		}
		return true
	})

	// Dump JSON to the given output.
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

// LogStream implements Receiver.
func (r *ReportReceiver) LogStream(stepId, logId string, _ Severity) (io.Writer, error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	buf := bytes.NewBuffer([]byte{})
	step, err := r.findStep(stepId)
	if err != nil {
		return nil, err
	}
	if _, ok := step.Logs[logId]; ok {
		return nil, fmt.Errorf("Step %s already has a log with ID %s", stepId, logId)
	}
	step.Logs[logId] = buf
	return buf, nil
}

// CloudLoggingReceiver is a Receiver which sends step metadata and logs to
// Cloud Logging.
type CloudLoggingReceiver struct {
	// logger is a handle to the Logger used for the entire test run.
	logger *logging.Logger
}

// NewCloudLoggingReceiver returns a CloudLoggingReceiver instance. This
// initializes Cloud Logging for the entire test run.
func NewCloudLoggingReceiver(logger *logging.Logger) (*CloudLoggingReceiver, error) {
	return &CloudLoggingReceiver{
		logger: logger,
	}, nil
}

// HandleMessage implements Receiver.
func (r *CloudLoggingReceiver) HandleMessage(m *Message) error {
	// TODO(borenet): When should we LogSync, or Flush? If the program
	// crashes or is killed, we'll want to have already flushed the logs.
	labels := map[string]string{}
	if m.StepId != "" {
		labels["stepId"] = m.StepId
	}
	r.logger.Log(logging.Entry{
		Payload:  m,
		Severity: logging.Debug,
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

// LogStream implements Receiver.
func (r *CloudLoggingReceiver) LogStream(stepId, logId string, severity Severity) (io.Writer, error) {
	return &cloudLogsWriter{
		logger: r.logger,
		labels: map[string]string{
			"logId":  logId,
			"stepId": stepId,
		},
		severity: severity.asCloudLoggingSeverity(),
	}, nil
}

// Close implements Receiver.
func (r *CloudLoggingReceiver) Close() error {
	return r.logger.Flush()
}
