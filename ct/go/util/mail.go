// Utility that contains methods for dealing with emails.
package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/metadata"

	"go.skia.org/infra/go/sklog"
)

var (
	emailClientId     string
	emailClientSecret string
)

func MailInit() {
	emailClientId = metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_ID))
	emailClientSecret = metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_SECRET))
	cachedGMailToken := metadata.Must(metadata.ProjectGet(GMAIL_CACHED_TOKEN))
	if err := ioutil.WriteFile(EmailTokenPath, []byte(cachedGMailToken), os.ModePerm); err != nil {
		sklog.Fatalf("Failed to cache token: %s", err)
	}
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
	gmail, err := email.NewGMail(emailClientId, emailClientSecret, EmailTokenPath)
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
	gmail, err := email.NewGMail(emailClientId, emailClientSecret, EmailTokenPath)
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
			"Please check the master log <a href='%s'>here</a> and the logs of triggered swarming tasks <a href='%s'>here</a>."+
			"<br/>Contact the admins %s for assistance.<br/><br/>",
		GetMasterLogLink(runID), fmt.Sprintf(SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID), CtAdmins)
}

func SendTaskStartEmail(recipients []string, taskName, runID, description string) error {
	emailSubject := fmt.Sprintf("%s cluster telemetry task has started (%s)", taskName, runID)
	masterLogLink := GetMasterLogLink(runID)
	swarmingLogsLink := fmt.Sprintf(SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID)

	viewActionMarkup, err := email.GetViewActionMarkup(swarmingLogsLink, "View Logs", "Direct link to the swarming logs")
	if err != nil {
		return fmt.Errorf("Failed to get view action markup: %s", err)
	}
	descriptionHtml := ""
	if description != "" {
		descriptionHtml = fmt.Sprintf("Run description: %s<br/><br/>", description)
	}
	bodyTemplate := `
	The %s queued task has started.<br/>
	%s
	You can watch the logs of the master <a href="%s">here</a> and the logs of triggered swarming tasks <a href="%s">here</a>.<br/>
	<b>Note:</b> Must be on Google corp to access the master logs.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, taskName, descriptionHtml, masterLogLink, swarmingLogsLink)
	if err := SendEmailWithMarkup(recipients, emailSubject, emailBody, viewActionMarkup); err != nil {
		return fmt.Errorf("Error while sending task start email: %s", err)
	}
	return nil
}

// GetSwarmingLogsLink returns HTML snippet that contains a href to the swarming logs.
func GetSwarmingLogsLink(runID string) string {
	return fmt.Sprintf("Swarming logs <a href='%s'>link</a>", fmt.Sprintf(SWARMING_RUN_ID_ALL_TASKS_LINK_TEMPLATE, runID))
}
