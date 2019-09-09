// Utility that contains methods for dealing with emails.
package util

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"strings"

	"go.skia.org/infra/go/email"
	skutil "go.skia.org/infra/go/util"
)

var (
	emailClientId     string
	emailClientSecret string
	emailTokenPath    string
)

type ClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}
type Installed struct {
	Installed ClientConfig `json:"installed"`
}

func MailInit(emailClientSecretFile, emailTokenCacheFile string) error {
	var cfg Installed
	err := skutil.WithReadFile(emailClientSecretFile, func(f io.Reader) error {
		return json.NewDecoder(f).Decode(&cfg)
	})
	if err != nil {
		return fmt.Errorf("Failed to read client secrets from %q: %s", emailClientSecretFile, err)
	}
	// Create a copy of the token cache file since mounted secrets are read-only
	// and the access token will need to be updated for the oauth2 flow.
	fout, err := ioutil.TempFile("", "")
	if err != nil {
		return fmt.Errorf("Unable to create temp file: %s", err)
	}
	err = skutil.WithReadFile(emailTokenCacheFile, func(fin io.Reader) error {
		_, err := io.Copy(fout, fin)
		if err != nil {
			err = fout.Close()
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("Failed to write token cache file from %q to %q: %s", emailTokenCacheFile, fout.Name(), err)
	}
	emailTokenCacheFile = fout.Name()
	emailClientId = cfg.Installed.ClientID
	emailClientSecret = cfg.Installed.ClientSecret
	emailTokenPath = emailTokenCacheFile

	return nil
}

// ParseEmails returns an array containing emails from the provided comma
// separated emails string.
func ParseEmails(emails string) []string {
	emailsArr := []string{}
	for _, email := range strings.Split(emails, ",") {
		emailsArr = append(emailsArr, strings.TrimSpace(email))
	}
	return emailsArr
}

// SendEmail sends an email with the specified header and body to the recipients.
func SendEmail(recipients []string, subject, body string) error {
	gmail, err := email.NewGMail(emailClientId, emailClientSecret, emailTokenPath)
	if err != nil {
		return fmt.Errorf("Could not initialize gmail object: %s", err)
	}
	if err := gmail.Send(CT_EMAIL_DISPLAY_NAME, recipients, subject, body); err != nil {
		return fmt.Errorf("Could not send email: %s", err)
	}

	return nil
}

// SendEmailWithMarkup sends an email with the specified header and body to the recipients. It also
// includes gmail markups.
// Documentation about markups supported in gmail are here: https://developers.google.com/gmail/markup/
// A go-to action example is here: https://developers.google.com/gmail/markup/reference/go-to-action
func SendEmailWithMarkup(recipients []string, subject, body, markup string) error {
	gmail, err := email.NewGMail(emailClientId, emailClientSecret, emailTokenPath)
	if err != nil {
		return fmt.Errorf("Could not initialize gmail object: %s", err)
	}
	if err := gmail.SendWithMarkup(CT_EMAIL_DISPLAY_NAME, recipients, subject, body, markup); err != nil {
		return fmt.Errorf("Could not send email with markup: %s", err)
	}

	return nil
}

func GetFailureEmailHtml(runID string) string {
	return fmt.Sprintf(
		"<br/>There were <b>failures</b> in the run. "+
			"Please check the logs of triggered swarming tasks <a href='%s'>here</a>."+
			"<br/>Contact the admins %s for assistance.<br/><br/>",
		fmt.Sprintf(SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID), CtAdmins)
}

func GetCTPerfEmailHtml(groupName string) string {
	if groupName == "" {
		return ""
	} else {
		return fmt.Sprintf(`
			<br/>See graphed data for your run on <a href='https://ct-perf.skia.org/e/?request_type=1'>ct-perf.skia.org</a> by selecting %s for group_name and then selecting a sub_result and/or test.
			<br/>Example calculated traces:
			<ul>
				<li>ave(filter("group_name=test_group_name&sub_result=rasterize_time__ms_"))</li>
				<li>norm(filter("group_name=test_group_name&sub_result=rasterize_time__ms_&test=http___amazon.co.uk"))</li>
			</ul>
			Documentation for Perf is available <a href='http://go/perf-user-doc'>here</a>.
			<br/><br/>`,
			html.EscapeString(groupName))
	}
}

func SendTaskStartEmail(taskId int64, recipients []string, taskName, runID, runDescription, additionalDescription string) error {
	emailSubject := fmt.Sprintf("%s cluster telemetry task has started (#%d)", taskName, taskId)
	swarmingLogsLink := fmt.Sprintf(SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID)

	viewActionMarkup, err := email.GetViewActionMarkup(swarmingLogsLink, "View Logs", "Direct link to the swarming logs")
	if err != nil {
		return fmt.Errorf("Failed to get view action markup: %s", err)
	}
	descriptionHtml := ""
	if runDescription != "" {
		descriptionHtml += fmt.Sprintf("Run description: %s<br/><br/>", runDescription)
	}
	if additionalDescription != "" {
		descriptionHtml += fmt.Sprintf("%s<br/><br/>", additionalDescription)
	}
	bodyTemplate := `
	The %s queued task has started.<br/>
	%s
	You can watch the logs of triggered swarming tasks <a href="%s">here</a>.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, taskName, descriptionHtml, swarmingLogsLink)
	if err := SendEmailWithMarkup(recipients, emailSubject, emailBody, viewActionMarkup); err != nil {
		return fmt.Errorf("Error while sending task start email: %s", err)
	}
	return nil
}

// SendTasksTerminatedEmail informs the recipients that their CT tasks were terminated and that
// they should reschedule.
func SendTasksTerminatedEmail(recipients []string) error {
	emailSubject := fmt.Sprintf("Cluster telemetry tasks were terminated (%s)", GetCurrentTs())
	body := `
	The Cluster telemetry server had to be restarted due to a maintenance issue.<br/>
	This caused all running tasks to be terminated.<br/><br/>
	Please reschedule your tasks. You can redo your tasks by clicking on the redo icon in the 'Runs History' page on http://ct.skia.org.<br/><br/>
	Sorry for the inconvenience!
	`

	if err := SendEmail(recipients, emailSubject, body); err != nil {
		return fmt.Errorf("Error while sending tasks termination email: %s", err)
	}
	return nil
}

// GetSwarmingLogsLink returns HTML snippet that contains a href to the swarming logs.
func GetSwarmingLogsLink(runID string) string {
	return fmt.Sprintf("Swarming logs <a href='%s'>link</a>", fmt.Sprintf(SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID))
}
