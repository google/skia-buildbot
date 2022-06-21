// Utility that contains methods for dealing with emails.
package util

import (
	"fmt"
	"html"
	"strings"

	ctutil "go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/email/go/emailclient"
	"go.skia.org/infra/go/email"
)

const (
	emailDisplayName = "Cluster Telemetry"

	emailFromAddress = "ct@skia.org"
)

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
	email := emailclient.New()
	if _, err := email.SendWithMarkup(emailDisplayName, emailFromAddress, recipients, subject, body, "", ""); err != nil {
		return fmt.Errorf("Could not send email: %s", err)
	}

	return nil
}

// SendEmailWithMarkup sends an email with the specified header and body to the recipients. It also
// includes gmail markups.
// Documentation about markups supported in gmail are here: https://developers.google.com/gmail/markup/
// A go-to action example is here: https://developers.google.com/gmail/markup/reference/go-to-action
func SendEmailWithMarkup(recipients []string, subject, body, markup string) error {
	email := emailclient.New()
	if _, err := email.SendWithMarkup(emailDisplayName, emailFromAddress, recipients, subject, body, markup, ""); err != nil {
		return fmt.Errorf("Could not send email with markup: %s", err)
	}

	return nil
}

func GetFailureEmailHtml(runID string) string {
	return fmt.Sprintf(
		"<br/>There were <b>failures</b> in the run. "+
			"Please check the logs of triggered swarming tasks <a href='%s'>here</a>."+
			"<br/>Contact the admins %s for assistance.<br/><br/>",
		fmt.Sprintf(ctutil.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID), ctutil.CtAdmins)
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
	swarmingLogsLink := fmt.Sprintf(ctutil.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID)

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
	emailSubject := fmt.Sprintf("Cluster telemetry tasks were terminated (%s)", ctutil.GetCurrentTs())
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
	return fmt.Sprintf("Swarming logs <a href='%s'>link</a>", fmt.Sprintf(ctutil.SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID))
}
