package alerting

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"skia.googlesource.com/buildbot.git/go/email"
)

func sendEmail(a *Alert, to []string, emailAuth *email.GMail) {
	glog.Infof("Sending email to %s: %s", to, a.Name)
	subject := fmt.Sprintf("Skia Alert: %s triggered at %s", a.Name, a.Triggered().String())
	body := "For more information see the alert server: <someurl>"
	err := emailAuth.Send(to, subject, body)
	if err != nil {
		glog.Errorf("Failed to send email: %s", err)
	}

}

func printAlert(a *Alert) {
	glog.Infof("ALERT: %s", a.Name)
}

func parseEmailAlert(str string, emailAuth *email.GMail) (func(*Alert), error) {
	split := strings.Split(str, ",")
	emails := []string{}
	for _, email := range split {
		emails = append(emails, strings.Trim(email, " "))
	}
	return func(a *Alert) {
		sendEmail(a, emails, emailAuth)
	}, nil
}

func parseActions(actionsInterface interface{}, emailAuth *email.GMail) ([]func(*Alert), error) {
	actionsList := []func(*Alert){printAlert}
	actionStrings := actionsInterface.([]interface{})
	for _, a := range actionStrings {
		str := a.(string)
		glog.Info(str)
		if strings.HasPrefix(str, "Email(") && strings.HasSuffix(str, ")") {
			f, err := parseEmailAlert(str[6:len(str)-1], emailAuth)
			if err != nil {
				return nil, err
			}
			actionsList = append(actionsList, f)
		} else if str == "Print" {
			// Do nothing; print is added by default.
		} else {
			return nil, fmt.Errorf("Unknown action: %q", str)
		}
	}
	return actionsList, nil
}
