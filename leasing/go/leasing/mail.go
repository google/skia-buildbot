/*
	Used by the Leasing Server to send emails.
*/

package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
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

func MailInit(tokenPath string) error {
	emailTokenPath := tokenPath
	emailClientId := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_ID))
	emailClientSecret := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_SECRET))
	cachedGMailToken := metadata.Must(metadata.ProjectGet(GMAIL_CACHED_TOKEN))
	if err := ioutil.WriteFile(emailTokenPath, []byte(cachedGMailToken), os.ModePerm); err != nil {
		return fmt.Errorf("Failed to cache token: %s", err)
	}
	var err error
	gmail, err = email.NewGMail(emailClientId, emailClientSecret, emailTokenPath)
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
	return append(recipients, trooper)
}

func SendStartEmail(taskOwner, swarmingServer, swarmingId, swarmingBot string) error {
	subject := fmt.Sprintf("Your leasing task is now active (id:%s)", swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has been picked up by the swarming bot %s.
		<br/><br/>
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
	body := fmt.Sprintf(bodyTemplate, GetSwarmingTaskLink(swarmingServer, swarmingId), swarmingBot, CONNECTION_INSTRUCTIONS_PAGE, fmt.Sprintf("%s%s", PROD_URI, MY_LEASES_URI))
	if err := gmail.Send(LEASING_EMAIL_DISPLAY_NAME, getRecipients(taskOwner), subject, body); err != nil {
		return fmt.Errorf("Could not send start email: %s", err)
	}
	return nil
}

func SendWarningEmail(taskOwner, swarmingServer, swarmingId string) error {
	subject := fmt.Sprintf("Your leasing task will expire in ~15mins (id:%s)", swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has less than 15 mins remaining.
		<br/><br/>
		You can expire or extend the lease time <a href="%s">here</a>.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, GetSwarmingTaskLink(swarmingServer, swarmingId), fmt.Sprintf("%s%s", PROD_URI, MY_LEASES_URI))
	if err := gmail.Send(LEASING_EMAIL_DISPLAY_NAME, getRecipients(taskOwner), subject, body); err != nil {
		return fmt.Errorf("Could not send warning email: %s", err)
	}
	return nil
}

func SendFailureEmail(taskOwner, swarmingServer, swarmingId, swarmingTaskState string) error {
	subject := fmt.Sprintf("Your leasing task unexpectedly completed (id:%s)", swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> unexpectedly ended with the state: %s.
		<br/><br/>
		You can reschedule another leasing task <a href="%s">here</a>.
		<br/>
		Contact the CC'ed trooper if you have any questions.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, GetSwarmingTaskLink(swarmingServer, swarmingId), swarmingTaskState, PROD_URI)
	if err := gmail.Send(LEASING_EMAIL_DISPLAY_NAME, getRecipients(taskOwner), subject, body); err != nil {
		return fmt.Errorf("Could not send failure email: %s", err)
	}
	return nil
}

func SendCompletionEmail(taskOwner, swarmingServer, swarmingId string) error {
	subject := fmt.Sprintf("Your leasing task has completed (id:%s)", swarmingId)
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has completed.
		<br/><br/>
		If needed, you can reschedule more leasing tasks <a href="%s">here</a>.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, GetSwarmingTaskLink(swarmingServer, swarmingId), PROD_URI)
	if err := gmail.Send(LEASING_EMAIL_DISPLAY_NAME, getRecipients(taskOwner), subject, body); err != nil {
		return fmt.Errorf("Could not send completion email: %s", err)
	}
	return nil
}
