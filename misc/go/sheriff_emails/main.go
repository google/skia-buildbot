// sheriff_emails is an application that emails the next sheriff every week.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/util"
)

const (
	NEXT_SHERIFF_JSON_URL = "http://skia-tree-status.appspot.com/next-sheriff"

	EXTRA_RECIPIENT = "rmistry@google.com"

	EMAIL_TEMPLATE = `
Hi %s,
<br/><br/>

You will be the Skia sheriff for the coming week (%s - %s).
<br/><br/>

Documentation for sheriffs is in https://skia.org/dev/sheriffing.
<br/><br/>

The schedule for sheriffs is in http://skia-tree-status.appspot.com/sheriff.
<br/><br/>

If you need to swap shifts with someone (because you are out sick or on vacation), please get approval from the person you want to swap with. Then send an email to skiabot@google.com to have someone make the change in the database (or directly ping rmistry@).
<br/><br/>

Please let skiabot@google.com know if you have any other questions.
<br/><br/>

Thanks!
`
)

var (
	emailTokenPath = flag.String("email_token_path", "", "The file where the email token can be found.")
)

// sendEmail sends an email with the specified header and body to the recipients.
func sendEmail(recipients []string, subject, body string) error {
	gmail, err := email.NewGMail(
		"292895568497-u2m421dk2htq171bfodi9qoqtb5smuea.apps.googleusercontent.com",
		"jv-g54CaPS783QV6H8SdagYn",
		*emailTokenPath)
	if err != nil {
		return fmt.Errorf("Could not initialize gmail object: %s", err)
	}
	if err := gmail.Send(recipients, subject, body); err != nil {
		return fmt.Errorf("Could not send email: %s", err)
	}
	return nil
}

func main() {
	common.Init()

	if *emailTokenPath == "" {
		glog.Error("Must specify --email_token_path")
		return
	}

	defer glog.Flush()

	res, err := http.Get(NEXT_SHERIFF_JSON_URL)
	if err != nil {
		glog.Fatalf("Could not HTTP Get: %s", err)
	}
	defer util.Close(res.Body)

	var jsonType map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&jsonType); err != nil {
		glog.Fatalf("Could not unmarshal JSON: %s", err)
	}
	sheriffEmail, _ := jsonType["username"].(string)
	sheriffUsername := strings.Split(string(sheriffEmail), "@")[0]

	emailBody := fmt.Sprintf(EMAIL_TEMPLATE, sheriffUsername, jsonType["schedule_start"], jsonType["schedule_end"])
	emailSubject := fmt.Sprintf("%s is the next Skia Sheriff", sheriffUsername)
	if err := sendEmail([]string{sheriffEmail, EXTRA_RECIPIENT}, emailSubject, emailBody); err != nil {
		glog.Fatalf("Error sending email to sheriff: %s", err)
	}
}
