package notifier

import (
	"bytes"
	"context"
	"html/template"
	"net/http"
	"time"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/email/go/emailclient"
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/notifier"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// Types of notification message sent by the roller. These can be
	// selected via notifier.Config.IncludeMsgTypes.
	MSG_TYPE_ISSUE_UPDATE         = "issue update"
	MSG_TYPE_LAST_N_FAILED        = "last n failed"
	MSG_TYPE_MODE_CHANGE          = "mode change"
	MSG_TYPE_NEW_FAILURE          = "new failure"
	MSG_TYPE_NEW_SUCCESS          = "new success"
	MSG_TYPE_ROLL_CREATION_FAILED = "cl creation failed"
	MSG_TYPE_SAFETY_THROTTLE      = "safety throttle"
	MSG_TYPE_STRATEGY_CHANGE      = "strategy change"
	MSG_TYPE_SUCCESS_THROTTLE     = "success throttle"
	MSG_TYPE_TOO_MANY_CLS         = "too many CLs"

	// Templates for messages sent by the roller.
	subjectIssueUpdate = "The {{.ChildName}} into {{.ParentName}} AutoRoller has uploaded issue {{.IssueID}}"

	bodyModeChange    = "{{.User}} changed the mode to \"{{.Mode}}\" with message: {{.Message}}"
	subjectModeChange = "The {{.ChildName}} into {{.ParentName}} AutoRoller mode was changed"

	subjectNewFailure = "The {{.ChildName}} into {{.ParentName}} roll has failed (issue {{.IssueID}})"
	bodyNewFailure    = "The most recent roll attempt failed, while the previous attempt succeeded: {{.IssueURL}}"

	subjectNewSuccess = "The {{.ChildName}} into {{.ParentName}} roll is successful again (issue {{.IssueID}})"
	bodyNewSuccess    = "The most recent roll attempt succeeded, while the previous attempt failed: {{.IssueURL}}"

	subjectLastNFailed = "The last {{.N}} {{.ChildName}} into {{.ParentName}} rolls have failed"
	bodyLastNFailed    = "The roll is failing consistently. Time to investigate. The most recent roll attempt is here: {{.IssueURL}}"

	bodyRollCreationFailed    = "The roller failed to create a CL with:\n{{.Message}}"
	subjectRollCreationFailed = "The {{.ChildName}} into {{.ParentName}} AutoRoller failed to create a CL"

	bodyStrategyChange    = "{{.User}} changed the next-roll-revision strategy to \"{{.Strategy}}\" with message: {{.Message}}"
	subjectStrategyChange = "The {{.ChildName}} into {{.ParentName}} AutoRoller next-roll-revision strategy was changed"

	subjectThrottled     = "The {{.ChildName}} into {{.ParentName}} AutoRoller is throttled"
	bodySafetyThrottled  = "The roller is throttled because it attempted to upload too many CLs in too short a time.  The roller will unthrottle at {{.ThrottledUntil}}."
	bodySuccessThrottled = "The roller is throttled because it is configured not to land too many rolls within a time period. The roller will unthrottle at {{.ThrottledUntil}}."

	subjectTooManyCLs = "The {{.ChildName}} into {{.ParentName}} has uploaded too many CLs to the same revision"
	bodyTooManyCLs    = "The roller has uploaded {{.N}} CLs to roll to revision {{.Revision}}.  It will not upload any more CLs until a new revision is available to roll."

	footer = "\n\nThe AutoRoll server is located here: {{.ServerURL}}"
)

var (
	subjectTmplIssueUpdate = template.Must(template.New("subjectIssueUpdate").Parse(subjectIssueUpdate))

	subjectTmplModeChange = template.Must(template.New("subjectModeChange").Parse(subjectModeChange))
	bodyTmplModeChange    = template.Must(template.New("bodyModeChange").Parse(bodyModeChange))

	subjectTmplNewFailure = template.Must(template.New("subjectNewFailure").Parse(subjectNewFailure))
	bodyTmplNewFailure    = template.Must(template.New("bodyNewFailure").Parse(bodyNewFailure))

	subjectTmplNewSuccess = template.Must(template.New("subjectNewSuccess").Parse(subjectNewSuccess))
	bodyTmplNewSuccess    = template.Must(template.New("bodyNewSuccess").Parse(bodyNewSuccess))

	subjectTmplLastNFailed = template.Must(template.New("subjectLastNFailed").Parse(subjectLastNFailed))
	bodyTmplLastNFailed    = template.Must(template.New("bodyLastNFailed").Parse(bodyLastNFailed))

	bodyTmplRollCreationFailed    = template.Must(template.New("bodyRollCreationFailed").Parse(bodyRollCreationFailed))
	subjectTmplRollCreationFailed = template.Must(template.New("subjectRollCreationFailed").Parse(subjectRollCreationFailed))

	subjectTmplStrategyChange = template.Must(template.New("subjectStrategyChange").Parse(subjectStrategyChange))
	bodyTmplStrategyChange    = template.Must(template.New("bodyStrategyChange").Parse(bodyStrategyChange))

	subjectTmplThrottled     = template.Must(template.New("subjectThrottled").Parse(subjectThrottled))
	bodyTmplSafetyThrottled  = template.Must(template.New("bodySafetyThrottled").Parse(bodySafetyThrottled))
	bodyTmplSuccessThrottled = template.Must(template.New("bodySuccessThrottled").Parse(bodySuccessThrottled))

	subjectTmplTooManyCLs = template.Must(template.New("subjectTooManyCLs").Parse(subjectTooManyCLs))
	bodyTmplTooManyCLs    = template.Must(template.New("bodyTooManyCLs").Parse(bodyTooManyCLs))

	footerTmpl = template.Must(template.New("footer").Parse(footer))

	protoToMsgType = map[config.NotifierConfig_MsgType]string{
		config.NotifierConfig_ISSUE_UPDATE:         MSG_TYPE_ISSUE_UPDATE,
		config.NotifierConfig_LAST_N_FAILED:        MSG_TYPE_LAST_N_FAILED,
		config.NotifierConfig_MODE_CHANGE:          MSG_TYPE_MODE_CHANGE,
		config.NotifierConfig_NEW_FAILURE:          MSG_TYPE_NEW_FAILURE,
		config.NotifierConfig_NEW_SUCCESS:          MSG_TYPE_NEW_SUCCESS,
		config.NotifierConfig_ROLL_CREATION_FAILED: MSG_TYPE_ROLL_CREATION_FAILED,
		config.NotifierConfig_SAFETY_THROTTLE:      MSG_TYPE_SAFETY_THROTTLE,
		config.NotifierConfig_STRATEGY_CHANGE:      MSG_TYPE_STRATEGY_CHANGE,
		config.NotifierConfig_SUCCESS_THROTTLE:     MSG_TYPE_SUCCESS_THROTTLE,
	}
	msgTypeToProto = map[string]config.NotifierConfig_MsgType{
		MSG_TYPE_ISSUE_UPDATE:         config.NotifierConfig_ISSUE_UPDATE,
		MSG_TYPE_LAST_N_FAILED:        config.NotifierConfig_LAST_N_FAILED,
		MSG_TYPE_MODE_CHANGE:          config.NotifierConfig_MODE_CHANGE,
		MSG_TYPE_NEW_FAILURE:          config.NotifierConfig_NEW_FAILURE,
		MSG_TYPE_NEW_SUCCESS:          config.NotifierConfig_NEW_SUCCESS,
		MSG_TYPE_ROLL_CREATION_FAILED: config.NotifierConfig_ROLL_CREATION_FAILED,
		MSG_TYPE_SAFETY_THROTTLE:      config.NotifierConfig_SAFETY_THROTTLE,
		MSG_TYPE_STRATEGY_CHANGE:      config.NotifierConfig_STRATEGY_CHANGE,
		MSG_TYPE_SUCCESS_THROTTLE:     config.NotifierConfig_SUCCESS_THROTTLE,
	}

	// Note that these really belong in the go/notifier package, but it doesn't
	// really make sense for that package to import the AutoRoller's config
	// package.  These values must be kept in sync with those from go/notifier.
	protoToLogLevel = map[config.NotifierConfig_LogLevel]notifier.Filter{
		config.NotifierConfig_SILENT:  notifier.FILTER_SILENT,
		config.NotifierConfig_ERROR:   notifier.FILTER_ERROR,
		config.NotifierConfig_WARNING: notifier.FILTER_WARNING,
		config.NotifierConfig_INFO:    notifier.FILTER_INFO,
		config.NotifierConfig_DEBUG:   notifier.FILTER_DEBUG,
	}
	logLevelToProto = map[notifier.Filter]config.NotifierConfig_LogLevel{
		notifier.FILTER_SILENT:  config.NotifierConfig_SILENT,
		notifier.FILTER_ERROR:   config.NotifierConfig_ERROR,
		notifier.FILTER_WARNING: config.NotifierConfig_WARNING,
		notifier.FILTER_INFO:    config.NotifierConfig_INFO,
		notifier.FILTER_DEBUG:   config.NotifierConfig_DEBUG,
	}
)

// tmplVars is a struct which contains information used to fill
// text templates in the Subject and Body fields of messages.
type tmplVars struct {
	ChildName      string
	IssueID        string
	IssueURL       string
	Mode           string
	Message        string
	N              int
	ParentName     string
	Revision       string
	ServerURL      string
	Strategy       string
	ThrottledUntil string
	User           string
}

// AutoRollNotifier is a struct used for sending notifications from an
// AutoRoller. It is a convenience wrapper around notifier.Router.
type AutoRollNotifier struct {
	childName    string
	client       *http.Client
	configReader chatbot.ConfigReader
	emailer      emailclient.Client
	n            *notifier.Router
	parentName   string
	serverURL    string
}

// Return an AutoRollNotifier instance.
func New(ctx context.Context, childName, parentName, serverURL string, client *http.Client, emailer emailclient.Client, chatBotConfigReader chatbot.ConfigReader, configs []*notifier.Config) (*AutoRollNotifier, error) {
	n := &AutoRollNotifier{
		childName:    childName,
		client:       client,
		configReader: chatBotConfigReader,
		emailer:      emailer,
		parentName:   parentName,
		serverURL:    serverURL,
	}
	if err := n.ReloadConfigs(ctx, configs); err != nil {
		return nil, err
	}
	return n, nil
}

func (a *AutoRollNotifier) ReloadConfigs(ctx context.Context, configs []*notifier.Config) error {
	// Create a new router and add the specified configs to it.
	n := notifier.NewRouter(a.client, a.emailer, a.configReader)
	if err := n.AddFromConfigs(ctx, configs); err != nil {
		return err
	}
	a.n = n
	return nil
}

// Return the underlying notifier.Router.
func (a *AutoRollNotifier) Router() *notifier.Router {
	return a.n
}

// Send a message.
func (a *AutoRollNotifier) send(ctx context.Context, vars *tmplVars, subjectTmpl, bodyTmpl *template.Template, severity notifier.Severity, msgType string) {
	vars.ChildName = a.childName
	vars.ParentName = a.parentName
	vars.ServerURL = a.serverURL
	var subjectBytes bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBytes, vars); err != nil {
		sklog.Errorf("Failed to send notification; failed to execute subject template: %s", err)
		return
	}
	var bodyBytes bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBytes, vars); err != nil {
		sklog.Errorf("Failed to send notification; failed to execute body template: %s", err)
		return
	}
	if err := footerTmpl.Execute(&bodyBytes, vars); err != nil {
		sklog.Errorf("Failed to send notification; failed to execute footer template: %s", err)
		return
	}
	sklog.Infof("Sending notification (%s; %s): %s\n\n%s", severity.String(), msgType, subjectBytes.String(), bodyBytes.String())
	if err := a.n.Send(ctx, &notifier.Message{
		Subject:  subjectBytes.String(),
		Body:     bodyBytes.String(),
		Severity: severity,
		Type:     msgType,
	}); err != nil {
		// We don't want to block the roller on failure to send
		// notifications. Log the error and move on.
		sklog.Error(err)
	}
}

// Send an issue update message.
func (a *AutoRollNotifier) SendIssueUpdate(ctx context.Context, id, url, msg string) {
	bodyTmpl, err := template.New("body").Parse(msg)
	if err != nil {
		sklog.Errorf("Failed to send notification; failed to parse template from message: %s", err)
		return
	}
	a.send(ctx, &tmplVars{
		IssueID:  id,
		IssueURL: url,
	}, subjectTmplIssueUpdate, bodyTmpl, notifier.SEVERITY_INFO, MSG_TYPE_ISSUE_UPDATE)
}

// Send a mode change message.
func (a *AutoRollNotifier) SendModeChange(ctx context.Context, user, mode, message string) {
	a.send(ctx, &tmplVars{
		Message: message,
		Mode:    mode,
		User:    user,
	}, subjectTmplModeChange, bodyTmplModeChange, notifier.SEVERITY_WARNING, MSG_TYPE_MODE_CHANGE)
}

// Send a notification that creation of a roll failed.
func (a *AutoRollNotifier) SendRollCreationFailed(ctx context.Context, err error) {
	a.send(ctx, &tmplVars{
		Message: err.Error(),
	}, subjectTmplRollCreationFailed, bodyTmplRollCreationFailed, notifier.SEVERITY_ERROR, MSG_TYPE_ROLL_CREATION_FAILED)
}

// Send a strategy change message.
func (a *AutoRollNotifier) SendStrategyChange(ctx context.Context, user, strategy, message string) {
	a.send(ctx, &tmplVars{
		Message:  message,
		Strategy: strategy,
		User:     user,
	}, subjectTmplStrategyChange, bodyTmplStrategyChange, notifier.SEVERITY_WARNING, MSG_TYPE_STRATEGY_CHANGE)
}

// Send a notification that the roller is safety-throttled.
func (a *AutoRollNotifier) SendSafetyThrottled(ctx context.Context, until time.Time) {
	a.send(ctx, &tmplVars{
		ThrottledUntil: until.Format(time.RFC1123),
	}, subjectTmplThrottled, bodyTmplSafetyThrottled, notifier.SEVERITY_ERROR, MSG_TYPE_SAFETY_THROTTLE)
}

// Send a notification that the roller is success-throttled.
func (a *AutoRollNotifier) SendSuccessThrottled(ctx context.Context, until time.Time) {
	a.send(ctx, &tmplVars{
		ThrottledUntil: until.Format(time.RFC1123),
	}, subjectTmplThrottled, bodyTmplSuccessThrottled, notifier.SEVERITY_INFO, MSG_TYPE_SUCCESS_THROTTLE)
}

// Send a notification that the most recent roll succeeded when the roll before
// it failed.
func (a *AutoRollNotifier) SendNewSuccess(ctx context.Context, id, url string) {
	a.send(ctx, &tmplVars{
		IssueID:  id,
		IssueURL: url,
	}, subjectTmplNewSuccess, bodyTmplNewSuccess, notifier.SEVERITY_WARNING, MSG_TYPE_NEW_SUCCESS)
}

// Send a notification that the most recent roll failed when the roll before
// it succeeded.
func (a *AutoRollNotifier) SendNewFailure(ctx context.Context, id, url string) {
	a.send(ctx, &tmplVars{
		IssueID:  id,
		IssueURL: url,
	}, subjectTmplNewFailure, bodyTmplNewFailure, notifier.SEVERITY_WARNING, MSG_TYPE_NEW_FAILURE)
}

// Send a notification that the last N roll attempts have failed.
func (a *AutoRollNotifier) SendLastNFailed(ctx context.Context, n int, url string) {
	a.send(ctx, &tmplVars{
		IssueURL: url,
		N:        n,
	}, subjectTmplLastNFailed, bodyTmplLastNFailed, notifier.SEVERITY_ERROR, MSG_TYPE_LAST_N_FAILED)
}

// Send a notification that too many CLs have been created to roll to the same
// revision.
func (a *AutoRollNotifier) SendTooManyCLs(ctx context.Context, numCLs int, rev string) {
	a.send(ctx, &tmplVars{
		N:        numCLs,
		Revision: rev,
	}, subjectTmplTooManyCLs, bodyTmplTooManyCLs, notifier.SEVERITY_ERROR, MSG_TYPE_TOO_MANY_CLS)
}

// ConfigToProto converts a notifier.Config to a config.NotifierConfig.
func ConfigToProto(cfg *notifier.Config) (*config.NotifierConfig, error) {
	rv := &config.NotifierConfig{
		Subject: cfg.Subject,
	}

	if cfg.Filter != "" {
		filter, err := notifier.ParseFilter(cfg.Filter)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv.LogLevel = logLevelToProto[filter]
	} else {
		for _, msgType := range cfg.IncludeMsgTypes {
			rv.MsgType = append(rv.MsgType, msgTypeToProto[msgType])
		}
	}

	if cfg.Chat != nil {
		rv.Config = &config.NotifierConfig_Chat{
			Chat: &config.ChatNotifierConfig{
				RoomId: cfg.Chat.RoomID,
			},
		}
	} else if cfg.Email != nil {
		rv.Config = &config.NotifierConfig_Email{
			Email: &config.EmailNotifierConfig{
				Emails: cfg.Email.Emails,
			},
		}
	} else if cfg.Monorail != nil {
		rv.Config = &config.NotifierConfig_Monorail{
			Monorail: &config.MonorailNotifierConfig{
				Project:    cfg.Monorail.Project,
				Owner:      cfg.Monorail.Owner,
				Cc:         cfg.Monorail.CC,
				Components: cfg.Monorail.Components,
				Labels:     cfg.Monorail.Labels,
			},
		}
	} else if cfg.PubSub != nil {
		rv.Config = &config.NotifierConfig_Pubsub{
			Pubsub: &config.PubSubNotifierConfig{
				Topic: cfg.PubSub.Topic,
			},
		}
	}

	return rv, nil
}

// ProtoToConfig converts a config.NotifierConfig to a notifier.Config.
func ProtoToConfig(cfg *config.NotifierConfig) *notifier.Config {
	rv := &notifier.Config{
		Subject: cfg.Subject,
	}

	if len(cfg.MsgType) > 0 {
		for _, msgType := range cfg.MsgType {
			rv.IncludeMsgTypes = append(rv.IncludeMsgTypes, protoToMsgType[msgType])
		}
	} else {
		rv.Filter = protoToLogLevel[cfg.LogLevel].String()
	}

	if chat, ok := cfg.Config.(*config.NotifierConfig_Chat); ok {
		rv.Chat = &notifier.ChatNotifierConfig{
			RoomID: chat.Chat.RoomId,
		}
	} else if email, ok := cfg.Config.(*config.NotifierConfig_Email); ok {
		rv.Email = &notifier.EmailNotifierConfig{
			Emails: email.Email.Emails,
		}
	} else if monorail, ok := cfg.Config.(*config.NotifierConfig_Monorail); ok {
		rv.Monorail = &notifier.MonorailNotifierConfig{
			Project:    monorail.Monorail.Project,
			Owner:      monorail.Monorail.Owner,
			CC:         monorail.Monorail.Cc,
			Components: monorail.Monorail.Components,
			Labels:     monorail.Monorail.Labels,
		}
	} else if pubsub, ok := cfg.Config.(*config.NotifierConfig_Pubsub); ok {
		rv.PubSub = &notifier.PubSubNotifierConfig{
			Topic: pubsub.Pubsub.Topic,
		}
	}

	return rv
}
