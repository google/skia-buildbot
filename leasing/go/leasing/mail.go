/*
	Used by the Leasing Server to send emails.
*/

package main

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/rotations"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	leasingEmailDisplayName = "Leasing Server"

	connectionInstructionsPage = "https://skia.org/dev/testing/swarmingbots#connecting-to-swarming-bots"
)

var (
	mail email.Client

	httpClient = httputils.NewTimeoutClient()
)

func MailInit(ctx context.Context) error {
	emailer, err := email.NewClient(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	mail = emailer
	return nil
}

func getRecipients(taskOwner string) []string {
	// Figure out the list of recipients.
	recipients := []string{taskOwner}
	gardeners, err := rotations.FromURL(httpClient, rotations.InfraGardenerURL)
	if err != nil {
		sklog.Errorf("Could not get gardener email: %s", err)
		return recipients
	}
	// Make sure rmistry@ is included on all emails for now.
	recipients = append(recipients, "rmistry@google.com")
	return append(recipients, gardeners...)
}

// SendStartEmail sends an email notifying user that the leasing task has started.
// It returns the email's threadingReference to use for threading followup emails.
func SendStartEmail(ownerEmail, swarmingServer, swarmingId, swarmingBot, TaskIdForIsolates string) (string, error) {
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
		Please see <a href="%s">this page</a> for instructions on how to connect to the bot.
		<br/>
		Contact the CC'ed Infra Gardener if you have any questions.
		<br/><br/>
		You can expire or extend the lease time <a href="%s">here</a>.
		<br/>
		Another email will be sent 15 mins before the lease end time.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, GetSwarmingBotLink(swarmingServer, swarmingBot), swarmingBot, sectionAboutIsolates, connectionInstructionsPage, fmt.Sprintf("https://%s%s", *host, myLeasesURI))
	markup, err := getSwarmingLinkMarkup(taskLink)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to get view action markup")
	}
	return email.SendWithMarkup(context.TODO(), mail, getRecipients(ownerEmail), subject, body, markup, "")
}

func SendWarningEmail(ownerEmail, swarmingServer, swarmingId, swarmingBot, threadingReference string) error {
	subject := getSubject(ownerEmail, swarmingBot, swarmingId)
	taskLink := GetSwarmingTaskLink(swarmingServer, swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has less than 15 mins remaining.
		<br/><br/>
		You can expire or extend the lease time <a href="%s">here</a>.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, fmt.Sprintf("https://%s%s", *host, myLeasesURI))
	markup, err := getSwarmingLinkMarkup(taskLink)
	if err != nil {
		return skerr.Wrapf(err, "failed to get view action markup")
	}
	if _, err := email.SendWithMarkup(context.TODO(), mail, getRecipients(ownerEmail), subject, body, markup, threadingReference); err != nil {
		return skerr.Wrapf(err, "could not send warning email")
	}
	return nil
}

func SendFailureEmail(ownerEmail, swarmingServer, swarmingId, swarmingBot, swarmingTaskState, threadingReference string) error {
	subject := getSubject(ownerEmail, swarmingBot, swarmingId)
	taskLink := GetSwarmingTaskLink(swarmingServer, swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> unexpectedly ended with the state: %s.
		<br/><br/>
		You can reschedule another leasing task <a href="https://%s">here</a>.
		<br/>
		Contact the CC'ed Infra Gardener if you have any questions.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, swarmingTaskState, *host)
	markup, err := getSwarmingLinkMarkup(taskLink)
	if err != nil {
		return skerr.Wrapf(err, "failed to get view action markup")
	}
	if _, err := email.SendWithMarkup(context.TODO(), mail, getRecipients(ownerEmail), subject, body, markup, threadingReference); err != nil {
		return skerr.Wrapf(err, "could not send failure email")
	}
	return nil
}

func SendExtensionEmail(ownerEmail, swarmingServer, swarmingId, swarmingBot, threadingReference string, durationHrs int) error {
	subject := getSubject(ownerEmail, swarmingBot, swarmingId)
	taskLink := GetSwarmingTaskLink(swarmingServer, swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has been extended by %dhr.
		<br/><br/>
		If needed, you can reschedule more leasing tasks <a href="https://%s">here</a>.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, durationHrs, *host)
	markup, err := getSwarmingLinkMarkup(taskLink)
	if err != nil {
		return skerr.Wrapf(err, "failed to get view action markup")
	}
	if _, err := email.SendWithMarkup(context.TODO(), mail, getRecipients(ownerEmail), subject, body, markup, threadingReference); err != nil {
		return skerr.Wrapf(err, "could not send completion email")
	}
	return nil
}

func SendCompletionEmail(ownerEmail, swarmingServer, swarmingId, swarmingBot, threadingReference string) error {
	subject := getSubject(ownerEmail, swarmingBot, swarmingId)
	taskLink := GetSwarmingTaskLink(swarmingServer, swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has completed.
		<br/><br/>
		If needed, you can reschedule more leasing tasks <a href="https://%s">here</a>.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, *host)
	markup, err := getSwarmingLinkMarkup(taskLink)
	if err != nil {
		return skerr.Wrapf(err, "failed to get view action markup")
	}
	if _, err := email.SendWithMarkup(context.TODO(), mail, getRecipients(ownerEmail), subject, body, markup, threadingReference); err != nil {
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
