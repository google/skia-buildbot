// sheriff_emails is an application that emails the next sheriff every week.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
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
Hi {{.SheriffName}},
<br/><br/>

You will be the {{.SheriffType}} for the coming week ({{.ScheduleStart}} - {{.ScheduleEnd}}).
<br/><br/>

Documentation for {{.SheriffType}}s is in {{.SheriffDoc}}.
<br/><br/>

The schedule for {{.SheriffType}}s is in {{.SheriffSchedules}}.
<br/><br/>

If you need to swap shifts with someone (because you are out sick or on vacation), please get approval from the person you want to swap with. Then send an email to skiabot@google.com to have someone make the change in the database (or directly ping rmistry@).
<br/><br/>

Please let skiabot@google.com know if you have any other questions.
<br/><br/>

Thanks!
`
)

type ShiftType struct {
	shiftName           string
	schedulesLink       string
	documentationLink   string
	nextSheriffEndpoint string
}

var (
	emailTokenPath   = flag.String("email_token_path", "", "The file where the email token can be found.")
	skiaSheriffShift = &ShiftType{shiftName: "Skia Sheriff", schedulesLink: "http://skia-tree-status.appspot.com/sheriff", documentationLink: "https://skia.org/dev/sheriffing", nextSheriffEndpoint: "http://skia-tree-status.appspot.com/next-sheriff"}
	gpuWranglerShift = &ShiftType{shiftName: "GPU Wrangler", schedulesLink: "http://skia-tree-status.appspot.com/gpu-sheriff", documentationLink: "https://skia.org/dev/sheriffing/gpu", nextSheriffEndpoint: "http://skia-tree-status.appspot.com/next-gpu-sheriff"}
	trooperShift     = &ShiftType{shiftName: "Infra Trooper", schedulesLink: "http://skia-tree-status.appspot.com/trooper", documentationLink: "https://skia.org/dev/sheriffing/trooper", nextSheriffEndpoint: "http://skia-tree-status.appspot.com/next-trooper"}
	allShiftTypes    = []*ShiftType{skiaSheriffShift, gpuWranglerShift, trooperShift}
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

	for _, shiftType := range allShiftTypes {

		res, err := http.Get(shiftType.nextSheriffEndpoint)
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

		emailTemplateParsed := template.Must(template.New("sheriff_email").Parse(EMAIL_TEMPLATE))
		emailBytes := new(bytes.Buffer)
		if err := emailTemplateParsed.Execute(emailBytes, struct {
			SheriffName      string
			SheriffType      string
			SheriffSchedules string
			SheriffDoc       string
			ScheduleStart    string
			ScheduleEnd      string
		}{
			SheriffName:      sheriffUsername,
			SheriffType:      shiftType.shiftName,
			SheriffSchedules: shiftType.schedulesLink,
			SheriffDoc:       shiftType.documentationLink,
			ScheduleStart:    jsonType["schedule_start"].(string),
			ScheduleEnd:      jsonType["schedule_end"].(string),
		}); err != nil {
			glog.Errorf("Failed to execute template: %s", err)
			return
		}

		emailSubject := fmt.Sprintf("%s is the next %s", sheriffUsername, shiftType.shiftName)
		if err := sendEmail([]string{sheriffEmail, EXTRA_RECIPIENT}, emailSubject, emailBytes.String()); err != nil {
			glog.Fatalf("Error sending email to sheriff: %s", err)
		}
	}
}
