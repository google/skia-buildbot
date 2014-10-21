package alerting

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/influxdb/influxdb/client"
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

// Snooze the given alert until the given time.
func (am *AlertManager) Snooze(name string, until time.Time) {
	am.alerts[name].snooze(until)
}

// Dismiss the given alert.
func (am *AlertManager) Dismiss(name string) {
	am.alerts[name].dismiss()
}

func (am *AlertManager) loop() {
	for _ = range time.Tick(am.tickInterval) {
		for _, a := range am.alerts {
			go a.tick()
		}
	}
}

// NewAlertManager creates and returns an AlertManager instance.
func NewAlertManager(dbClient *client.Client, alertsCfg string, tickInterval time.Duration) (*AlertManager, error) {
	// TODO(borenet): Dynamically create these actions.
	actions := map[string]func(string){
		"Print": func(s string) { glog.Infof("ALERT: %v", s) },
	}

	// Create the rules.
	alerts, err := makeAlerts(alertsCfg, actions, dbClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to create alerts: %v", err)
	}
	alertMap := map[string]*Alert{}
	for _, a := range alerts {
		alertMap[a.Name] = a
	}
	am := AlertManager{
		alerts:       alertMap,
		tickInterval: tickInterval,
	}
	go am.loop()
	return &am, nil
}
