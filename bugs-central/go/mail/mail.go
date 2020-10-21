/*
	Used by the Bugs Central Server to send emails.
*/

package mail

import (
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
)

const (
	bugsCentralEmailDisplayName = "Skia Bugs Central"

	gmailCachedToken = "bugs_central_gmail_cached_token"
)

var (
	gmail *email.GMail

	httpClient = httputils.NewTimeoutClient()
)

func MailInit(emailClientId, emailClientSecret, tokenFile string) error {
	// var err error
	// gmail, err = email.NewGMail(emailClientId, emailClientSecret, tokenFile)
	// if err != nil {
	// 	return fmt.Errorf("Could not initialize gmail object: %s", err)
	// }

	return nil
}

// TODO(rmistry): Complete mail implementation and use it.
