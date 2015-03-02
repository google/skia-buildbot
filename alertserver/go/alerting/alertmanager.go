package alerting

import (
	"fmt"
	"time"

	"github.com/skia-dev/influxdb/client"
	"go.skia.org/infra/go/email"
)

// AlertManager is the primary point of interaction with Alert objects.
type AlertManager struct {
	rules        map[string]*Rule
	tickInterval time.Duration
}

// Alerts returns a slice of the Alert instances held by this AlertManager.
func (am *AlertManager) Alerts() []*Alert {
	out := []*Alert{}
	for _, r := range am.rules {
		if r.activeAlert != nil {
			out = append(out, r.activeAlert)
		}
	}
	return out
}

// Rules returns a slice of Rule objects for this AlertManager.
func (am *AlertManager) Rules() []*Rule {
	out := []*Rule{}
	for _, r := range am.rules {
		out = append(out, r)
	}
	return out
}

// Contains indicates whether this AlertManager has an Alert with the given ID.
func (am *AlertManager) Contains(id string) bool {
	_, contains := am.rules[id]
	return contains
}

// Snooze the given alert until the given time.
func (am *AlertManager) Snooze(id string, until time.Time, user string) {
	r := am.rules[id]
	if r.activeAlert != nil {
		r.activeAlert.snoozedUntil = until
		r.activeAlert.addComment(&Comment{
			Time:    time.Now().UTC(),
			User:    user,
			Message: fmt.Sprintf("Snoozed until %s", until.String()),
		})
	}
}

// Unsnooze the given alert.
func (am *AlertManager) Unsnooze(id, user string) {
	r := am.rules[id]
	if r.activeAlert != nil {
		r.activeAlert.snoozedUntil = time.Time{}
		r.activeAlert.addComment(&Comment{
			Time:    time.Now().UTC(),
			User:    user,
			Message: fmt.Sprintf("Unsnoozed"),
		})
	}
}

// Dismiss the given alert.
func (am *AlertManager) Dismiss(id, user string) {
	r := am.rules[id]
	if r.activeAlert != nil {
		r.activeAlert.addComment(&Comment{
			Time:    time.Now().UTC(),
			User:    user,
			Message: fmt.Sprintf("Dismissed"),
		})
	}
	r.activeAlert = nil
}

// Add a comment to the given alert.
func (am *AlertManager) AddComment(id, user, msg string) {
	r := am.rules[id]
	if r.activeAlert != nil {
		r.activeAlert.addComment(&Comment{
			Time:    time.Now().UTC(),
			User:    user,
			Message: msg,
		})
	}
}

func (am *AlertManager) loop() {
	for _ = range time.Tick(am.tickInterval) {
		for _, a := range am.rules {
			go a.tick()
		}
	}
}

// NewAlertManager creates and returns an AlertManager instance.
func NewAlertManager(dbClient *client.Client, alertsCfg string, tickInterval time.Duration, emailAuth *email.GMail, testing bool) (*AlertManager, error) {
	// Create the rules.
	rules, err := makeRules(alertsCfg, dbClient, emailAuth, testing)
	if err != nil {
		return nil, fmt.Errorf("Failed to create alerts: %v", err)
	}
	ruleMap := map[string]*Rule{}
	for _, r := range rules {
		if _, contains := ruleMap[r.Id]; contains {
			return nil, fmt.Errorf("Alert ID collision.")
		}
		ruleMap[r.Id] = r
	}
	am := AlertManager{
		rules:        ruleMap,
		tickInterval: tickInterval,
	}
	go am.loop()
	return &am, nil
}
