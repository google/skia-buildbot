package alerting

import (
	"fmt"
	"time"

	"github.com/influxdb/influxdb/client"
	"skia.googlesource.com/buildbot.git/go/email"
)

// AlertManager is the primary point of interaction with Alert objects.
type AlertManager struct {
	alerts       map[string]*Alert
	tickInterval time.Duration
}

// Alerts returns a slice of the Alert instances held by this AlertManager.
func (am *AlertManager) Alerts() []*Alert {
	out := []*Alert{}
	for _, a := range am.alerts {
		out = append(out, a)
	}
	return out
}

// Contains indicates whether this AlertManager has an Alert with the given ID.
func (am *AlertManager) Contains(id string) bool {
	_, contains := am.alerts[id]
	return contains
}

// Snooze the given alert until the given time.
func (am *AlertManager) Snooze(id string, until time.Time) {
	am.alerts[id].snooze(until)
}

// Unsnooze the given alert.
func (am *AlertManager) Unsnooze(id string) {
	am.alerts[id].unsnooze()
}

// Dismiss the given alert.
func (am *AlertManager) Dismiss(id string) {
	am.alerts[id].dismiss()
}

func (am *AlertManager) loop() {
	for _ = range time.Tick(am.tickInterval) {
		for _, a := range am.alerts {
			go a.tick()
		}
	}
}

// NewAlertManager creates and returns an AlertManager instance.
func NewAlertManager(dbClient *client.Client, alertsCfg string, tickInterval time.Duration, emailAuth *email.GMail, testing bool) (*AlertManager, error) {
	// Create the rules.
	alerts, err := makeAlerts(alertsCfg, dbClient, emailAuth, testing)
	if err != nil {
		return nil, fmt.Errorf("Failed to create alerts: %v", err)
	}
	alertMap := map[string]*Alert{}
	for _, a := range alerts {
		if _, contains := alertMap[a.Id]; contains {
			return nil, fmt.Errorf("Alert ID collision.")
		}
		alertMap[a.Id] = a
	}
	am := AlertManager{
		alerts:       alertMap,
		tickInterval: tickInterval,
	}
	go am.loop()
	return &am, nil
}
