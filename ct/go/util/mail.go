// Utility that contains methods for dealing with emails.
package util

import (
	"fmt"
	"strings"

	"skia.googlesource.com/buildbot.git/go/email"
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
