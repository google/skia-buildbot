/*
	Used by the Leasing Server to send emails.
*/

package main

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

const (
	LEASING_EMAIL_DISPLAY_NAME = "Leasing Server"

	GMAIL_CACHED_TOKEN = "leasing_gmail_cached_token"

	CONNECTION_INSTRUCTIONS_PAGE = "https://skia.org/dev/testing/swarmingbots#connecting-to-swarming-bots"
)

var (
	gmail *email.GMail

	httpClient = httputils.NewTimeoutClient()
)

func MailInit(emailClientId, emailClientSecret, tokenFile string) error {
	var err error
	gmail, err = email.NewGMail(emailClientId, emailClientSecret, tokenFile)
	if err != nil {
		return fmt.Errorf("Could not initialize gmail object: %s", err)
	}

	return nil
}

func getRecipients(taskOwner string) []string {
	// Figure out the list of recipients.
	recipients := []string{taskOwner}
	trooper, err := GetTrooperEmail(httpClient)
	if err != nil {
		sklog.Errorf("Could not get trooper email: %s", err)
		return recipients
	}
	// Make sure rmistry@ is included on all emails for now.
	recipients = append(recipients, "rmistry@google.com")
	return append(recipients, trooper)
}

func SendStartEmail(ownerEmail, swarmingServer, swarmingId, swarmingBot, TaskIdForIsolates string) error {
	sectionAboutIsolates := ""
	if TaskIdForIsolates != "" {
		sectionAboutIsolatesTemplate := `
			Isolates downloaded from the <a href="%s">specified task</a> will be available on the bot.<br/>
			See the stdout of your <a href="%s">leasing task</a> for location of the artifacts and the command to run.<br/><br/>
		`
		sectionAboutIsolates = fmt.Sprintf(sectionAboutIsolatesTemplate, GetSwarmingTaskLink(swarmingServer, TaskIdForIsolates), GetSwarmingTaskLink(swarmingServer, swarmingId))
	}

	subject := getSubject(ownerEmail, swarmingBot, swarmingId)
	taskLink := GetSwarmingTaskLink(swarmingServer, swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has been picked up by the swarming bot <a href="%s">%s</a>.
		<br/><br/>
		%s
		%s
		Please see <a href="%s">this page</a> for instructions on how to connect to the bot.
		<br/>
		Contact the CC'ed trooper if you have any questions.
		<br/><br/>
		You can expire or extend the lease time <a href="%s">here</a>.
		<br/>
		Another email will be sent 15 mins before the lease end time.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, GetSwarmingBotLink(swarmingServer, swarmingBot), swarmingBot, sectionAboutIsolates, CONNECTION_INSTRUCTIONS_PAGE, fmt.Sprintf("%s%s", PROD_URI, MY_LEASES_URI))
	markup, err := getSwarmingLinkMarkup(taskLink)
	if err != nil {
		return fmt.Errorf("Failed to get view action markup: %s", err)
	}
	if err := gmail.SendWithMarkup(LEASING_EMAIL_DISPLAY_NAME, getRecipients(ownerEmail), subject, body, markup); err != nil {
		return fmt.Errorf("Could not send start email: %s", err)
	}
	return nil
}

func SendWarningEmail(ownerEmail, swarmingServer, swarmingId, swarmingBot string) error {
	subject := getSubject(ownerEmail, swarmingBot, swarmingId)
	taskLink := GetSwarmingTaskLink(swarmingServer, swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has less than 15 mins remaining.
		<br/><br/>
		You can expire or extend the lease time <a href="%s">here</a>.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, fmt.Sprintf("%s%s", PROD_URI, MY_LEASES_URI))
	markup, err := getSwarmingLinkMarkup(taskLink)
	if err != nil {
		return fmt.Errorf("Failed to get view action markup: %s", err)
	}
	if err := gmail.SendWithMarkup(LEASING_EMAIL_DISPLAY_NAME, getRecipients(ownerEmail), subject, body, markup); err != nil {
		return fmt.Errorf("Could not send warning email: %s", err)
	}
	return nil
}

func SendFailureEmail(ownerEmail, swarmingServer, swarmingId, swarmingBot, swarmingTaskState string) error {
	subject := getSubject(ownerEmail, swarmingBot, swarmingId)
	taskLink := GetSwarmingTaskLink(swarmingServer, swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> unexpectedly ended with the state: %s.
		<br/><br/>
		You can reschedule another leasing task <a href="%s">here</a>.
		<br/>
		Contact the CC'ed trooper if you have any questions.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, swarmingTaskState, PROD_URI)
	markup, err := getSwarmingLinkMarkup(taskLink)
	if err != nil {
		return fmt.Errorf("Failed to get view action markup: %s", err)
	}
	if err := gmail.SendWithMarkup(LEASING_EMAIL_DISPLAY_NAME, getRecipients(ownerEmail), subject, body, markup); err != nil {
		return fmt.Errorf("Could not send failure email: %s", err)
	}
	return nil
}

func SendExtensionEmail(ownerEmail, swarmingServer, swarmingId, swarmingBot string, durationHrs int) error {
	subject := getSubject(ownerEmail, swarmingBot, swarmingId)
	taskLink := GetSwarmingTaskLink(swarmingServer, swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has been extended by %dhr.
		<br/><br/>
		If needed, you can reschedule more leasing tasks <a href="%s">here</a>.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, durationHrs, PROD_URI)
	markup, err := getSwarmingLinkMarkup(taskLink)
	if err != nil {
		return fmt.Errorf("Failed to get view action markup: %s", err)
	}
	if err := gmail.SendWithMarkup(LEASING_EMAIL_DISPLAY_NAME, getRecipients(ownerEmail), subject, body, markup); err != nil {
		return fmt.Errorf("Could not send completion email: %s", err)
	}
	return nil
}

func SendCompletionEmail(ownerEmail, swarmingServer, swarmingId, swarmingBot string) error {
	subject := getSubject(ownerEmail, swarmingBot, swarmingId)
	taskLink := GetSwarmingTaskLink(swarmingServer, swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has completed.
		<br/><br/>
		If needed, you can reschedule more leasing tasks <a href="%s">here</a>.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, PROD_URI)
	markup, err := getSwarmingLinkMarkup(taskLink)
	if err != nil {
		return fmt.Errorf("Failed to get view action markup: %s", err)
	}
	if err := gmail.SendWithMarkup(LEASING_EMAIL_DISPLAY_NAME, getRecipients(ownerEmail), subject, body, markup); err != nil {
		return fmt.Errorf("Could not send completion email: %s", err)
	}
	return nil
}

func getSubject(ownerEmail, swarmingBot, swarmingId string) string {
	subjectTemplate := "%s's leasing task for %s update (id:%s)"
	return fmt.Sprintf(subjectTemplate, getUsernameFromEmail(ownerEmail), swarmingBot, swarmingId)

}

func getSwarmingLinkMarkup(taskLink string) (string, error) {
	return email.GetViewActionMarkup(taskLink, "View Logs", "Direct link to the swarming task logs")
}

func getUsernameFromEmail(e string) string {
	return strings.Split(e, "@")[0]
}
