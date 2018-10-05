package test_automation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	PUBSUB_TOPIC          = "recipe-steps"
	PUBSUB_TOPIC_INTERNAL = "recipe-steps-internal"
)

// Receiver is an interface used to implement arbitrary receivers of step
// metadata, as steps are run.
type Receiver interface {
	// Handle the given message.
	HandleMessage(*Message) error
	LogStream(string, string) (io.Writer, error)
}

// DebugReceiver just dumps the messages straight to the log.
type DebugReceiver struct{}

// See documentation for Receiver interface.
func (r *DebugReceiver) HandleMessage(m *Message) error {
	switch m.Type {
	case MSG_TYPE_STEP_STARTED:
		sklog.Infof("STEP_STARTED: %s", m.StepId)
	case MSG_TYPE_STEP_FINISHED:
		sklog.Infof("STEP_FINISHED: %s", m.StepId)
	case MSG_TYPE_STEP_DATA:
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

// See documentation for Receiver interface.
func (r *DebugReceiver) LogStream(stepId, logId string) (io.Writer, error) {
	return os.Stdout, nil
}

// stepReport is a struct used to collect information about a given step.
type stepReport struct {
	*StepResult
	*StepProperties
	Data  []interface{} `json:"data,omitempty"`
	logs  map[string]*bytes.Buffer
	Steps []*stepReport `json:"steps,omitempty"`
}

// ReportReceiver collects all messages and generates a report when requested.
type ReportReceiver struct {
	root *stepReport
}

// Recurse through all steps, running the given function. If the function
// returns false, recursion stops.
func (s *stepReport) recurse(fn func(*stepReport) bool) bool {
	if keepGoing := fn(s); !keepGoing {
		return false
	}
	for _, sub := range s.Steps {
		if keepGoing := sub.recurse(fn); !keepGoing {
			return false
		}
	}
	return true
}

// Find the step with the given ID in our list. This helps in case messages
// arrive out of order.
func (s *stepReport) findStep(id string) (*stepReport, error) {
	var rv *stepReport
	s.recurse(func(s *stepReport) bool {
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
func (r *ReportReceiver) findStep(id string) (*stepReport, error) {
	if r.root == nil {
		return nil, fmt.Errorf("No steps!")
	}
	return r.root.findStep(id)
}

// See documentation for Receiver interface.
func (r *ReportReceiver) HandleMessage(m *Message) error {
	switch m.Type {
	case MSG_TYPE_STEP_STARTED:
		s := &stepReport{
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
		s.StepResult = m.Result
	case MSG_TYPE_STEP_DATA:
		s, err := r.findStep(m.StepId)
		if err != nil {
			return err
		}
		s.Data = append(s.Data, m.Data)
	}
	return nil
}

// Write the report in JSON format to the given Writer.
func (r *ReportReceiver) Report(w io.Writer) error {
	// Attach the logs to each step, replacing the log ID with the actual
	// log contents.
	r.root.recurse(func(s *stepReport) bool {
		for _, data := range s.Data {
			d, ok := data.(*execData)
			if ok {
				for logName, logId := range d.Logs {
					if logBuf, ok := s.logs[logId]; ok {
						d.Logs[logName] = logBuf.String()
					}
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
	_, err = w.Write(b)
	return err
}

// See documentation for Receiver interface.
func (r *ReportReceiver) LogStream(stepId, logId string) (io.Writer, error) {
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
func NewCloudLoggingReceiver(ctx context.Context, projectId, taskId, taskName string, ts oauth2.TokenSource) (*CloudLoggingReceiver, error) {
	logsClient, err := logging.NewClient(ctx, projectId, option.WithTokenSource(ts))
	if err != nil {
		return nil, err
	}
	labels := map[string]string{
		"taskId":   taskId,
		"taskName": taskName,
	}
	logger := logsClient.Logger(taskId, logging.CommonLabels(labels))
	sklog.Infof("Connected Cloud Logging; logs can be found here: https://pantheon.corp.google.com/logs/viewer?project=%s&resource=project%%2Fproject_id%%2F%s&logName=projects%%2F%s%%2Flogs%%2F%s", projectId, projectId, projectId, taskId)
	return &CloudLoggingReceiver{
		logger: logger,
	}, nil
}

// See documentation for Receiver interface.
func (r *CloudLoggingReceiver) HandleMessage(m *Message) error {
	// TODO(borenet): When should we LogSync, or Flush? If the program
	// crashes or is killed, we'll want to have already flushed the logs.
	r.logger.Log(logging.Entry{Payload: m})
	return nil
}

// cloudLogsWriter is an io.Writer which writes to Cloud Logging.
type cloudLogsWriter struct {
	logger *logging.Logger
	labels map[string]string
}

// See documentation for io.Writer.
func (w *cloudLogsWriter) Write(b []byte) (int, error) {
	// TODO(borenet): Should we buffer until we see a newline?
	// TODO(borenet): When should we LogSync, or Flush? If the program
	// crashes or is killed, we'll want to have already flushed the logs.
	w.logger.Log(logging.Entry{
		Labels:  w.labels,
		Payload: string(b),
	})
	return len(b), nil
}

// See documentation for Receiver interface.
func (r *CloudLoggingReceiver) LogStream(stepId, logId string) (io.Writer, error) {
	return &cloudLogsWriter{
		logger: r.logger,
		labels: map[string]string{
			"logId":  logId,
			"stepId": stepId,
		},
	}, nil
}

// Flush the underlying logger.
func (r *CloudLoggingReceiver) Flush() error {
	return r.logger.Flush()
}

// PubSubReceiver is a Receiver which sends step metadata through Cloud Pubsub.
// It does not send logs.
type PubSubReceiver struct {
	topic *pubsub.Topic
}

// NewPubSubReceiver returns a PubSubReceiver instance.
func NewPubSubReceiver(ctx context.Context, projectId string) (Receiver, error) {
	client, err := pubsub.NewClient(ctx, common.PROJECT_ID)
	if err != nil {
		return nil, err
	}
	// TODO(borenet): Determine whether we should use the internal or
	// external topic (or maybe support other topics).
	topic := client.Topic(PUBSUB_TOPIC)
	return &PubSubReceiver{
		topic: topic,
	}, nil
}

// See documentation for Receiver interface.
func (r *PubSubReceiver) HandleMessage(m *Message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("Failed to encode message: %s", err)
	}
	// Send the message synchronously.
	ctx := context.Background()
	_, err = r.topic.Publish(ctx, &pubsub.Message{
		Data: b,
	}).Get(ctx)
	return err
}

// See documentation for Receiver interface.
func (r *PubSubReceiver) LogStream(_, _ string) (io.Writer, error) {
	// PubSubReceiver does not send logs.
	return ioutil.Discard, nil
}
