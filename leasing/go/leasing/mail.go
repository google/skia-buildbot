package main

import (
	"fmt"

	"go.skia.org/infra/go/email"
)

const (
	LEASING_EMAIL_DISPLAY_NAME = "Leasing Server"
)

var (
	gmail *email.GMail
)

func MailInit(tokenPath string) error {
	emailTokenPath := tokenPath
	/*
		emailClientId := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_ID))
		emailClientSecret := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_SECRET))
		cachedGMailToken := metadata.Must(metadata.ProjectGet(GMAIL_CACHED_TOKEN))
		if err := ioutil.WriteFile(emailTokenPath, []byte(cachedGMailToken), os.ModePerm); err != nil {
			return fmt.Errorf("Failed to cache token: %s", err)
		}
	*/
	var err error
	gmail, err = email.NewGMail(emailClientId, emailClientSecret, emailTokenPath)
	if err != nil {
		return fmt.Errorf("Could not initialize gmail object: %s", err)
	}
	return nil
}

// Move this to util.
func getTrooperEmail() (string, error) {
	// http://skia-tree-status.appspot.com/current-trooper
	// {"username": "kjlubick@google.com", "schedule_start": "10/30", "schedule_end": "11/05"}
	return "rmistry@google.com", nil
}

func SendStartEmail(taskOwner, swarmingServer, swarmingId, swarmingBot string) error {
	recipients := []string{taskOwner}
	trooper, err := getTrooperEmail()
	if err != nil {
		return fmt.Errorf("Could not get trooper email:%s", err)
	}
	recipients = append(recipients, trooper)

	taskLink := fmt.Sprintf("https://%s/task?id=%s", swarmingServer, swarmingId)
	subject := fmt.Sprintf("Your leasing task is now active (%s)", swarmingId)
	// TODO(rmistry): Make my_leases const and use it here.
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has been picked up by the swarming bot %s.
		<br/><br/>
		Please see this document for instructions on how to connect to the bot.
		<br/><br/>
		You can expire or extend the lease time <a href="https://leasing.skia.org/my_leases">here</a>.
		<br/>
		Another email will be sent 15 mins before the lease end time.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, swarmingBot)
	if err := gmail.Send(LEASING_EMAIL_DISPLAY_NAME, recipients, subject, body); err != nil {
		return fmt.Errorf("Could not send email: %s", err)
	}
	return nil
}

func SendWarningEmail(taskOwner, swarmingServer, swarmingId string) error {
	recipients := []string{taskOwner}
	trooper, err := getTrooperEmail()
	if err != nil {
		return fmt.Errorf("Could not get trooper email:%s", err)
	}
	recipients = append(recipients, trooper)

	taskLink := fmt.Sprintf("https://%s/task?id=%s", swarmingServer, swarmingId)
	subject := fmt.Sprintf("Your leasing task will expire in ~15mins (%s)", swarmingId)
	// TODO(rmistry): Make my_leases const and use it here.
	bodyTemplate := `
		Your <a href="%s">leasing task</a> has less than 15 mins remaining.
		<br/><br/>
		You can expire or extend the lease time <a href="https://leasing.skia.org/my_leases">here</a>.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink)
	if err := gmail.Send(LEASING_EMAIL_DISPLAY_NAME, recipients, subject, body); err != nil {
		return fmt.Errorf("Could not send email: %s", err)
	}
	return nil
}

func SendFailureEmail(taskOwner, swarmingServer, swarmingId, swarmingTaskState string) error {
	recipients := []string{taskOwner}
	trooper, err := getTrooperEmail()
	if err != nil {
		return fmt.Errorf("Could not get trooper email:%s", err)
	}
	recipients = append(recipients, trooper)

	taskLink := fmt.Sprintf("https://%s/task?id=%s", swarmingServer, swarmingId)
	subject := fmt.Sprintf("Your leasing task unexpectedly completed (%s)", swarmingId)
	// TODO(rmistry): Make my_leases const and use it here.
	bodyTemplate := `
		Your <a href="%s">leasing task</a> unexpectedly ended with the state: %s.
		<br/><br/>
		You can reschedule a leasing task <a href="https://leasing.skia.org/>here</a>.
		<br/><br/>
		Thanks!
	`
	body := fmt.Sprintf(bodyTemplate, taskLink, swarmingTaskState)
	if err := gmail.Send(LEASING_EMAIL_DISPLAY_NAME, recipients, subject, body); err != nil {
		return fmt.Errorf("Could not send email: %s", err)
	}
	return nil
}
