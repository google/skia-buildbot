// Utility that contains methods for dealing with emails.
package util

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/email"
)

var (
	FailureEmailHtml = fmt.Sprintf(
		"<br/>There were <b>failures</b> in the run. "+
			"Please check the master log <a href='%s'>here</a> and the worker log <a href='%s'>here</a>."+
			"<br/>Contact the admins %s for assistance.<br/><br/>",
		MASTER_LOGSERVER_LINK, WORKER1_LOGSERVER_LINK, CtAdmins)
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
	gmail, err := email.NewGMail(
		"292895568497-u2m421dk2htq171bfodi9qoqtb5smuea.apps.googleusercontent.com",
		"jv-g54CaPS783QV6H8SdagYn",
		EmailTokenPath)
	if err != nil {
		return fmt.Errorf("Could not initialize gmail object: %s", err)
	}
	if err := gmail.Send(recipients, subject, body); err != nil {
		return fmt.Errorf("Could not send email: %s", err)
	}

	return nil
}

func SendTaskStartEmail(recipients []string, taskName string) error {
	emailSubject := taskName + " cluster telemetry task has started"

	bodyTemplate := `
	The %s queued task has started.<br/>
	You can watch the logs of the master <a href="%s">here</a> and the logs of a worker <a href="%s">here</a>.<br/>
	<b>Note:</b> Must be on Google corp to access the above logs.<br/><br/>
	Thanks!
	`
	emailBody := fmt.Sprintf(bodyTemplate, taskName, MASTER_LOGSERVER_LINK, WORKER1_LOGSERVER_LINK)
	if err := SendEmail(recipients, emailSubject, emailBody); err != nil {
		return fmt.Errorf("Error while sending task start email: %s", err)
	}
	return nil
}
