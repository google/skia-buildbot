package alerting

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/util"
)

const (
	NAG_MSG_TMPL = "This alert has been active for %s since the last update. Please verify that it is still valid and either fix the issue or dismiss/snooze the alert."
)

var (
	Manager *AlertManager = nil
)

// AlertManager is the primary point of interaction with Alert objects.
type AlertManager struct {
	activeAlerts map[int64]*Alert
	interrupt    chan bool
	mutex        sync.RWMutex
	tickInterval time.Duration
}

// Alerts returns a slice containing the currently active Alerts.
func (am *AlertManager) WriteActiveAlertsJson(w io.Writer, filter func(*Alert) bool) error {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	out := make([]*Alert, 0, len(am.activeAlerts))
	ids := make([]int64, 0, len(am.activeAlerts))
	for _, a := range am.activeAlerts {
		if filter(a) {
			ids = append(ids, a.Id)
		}
	}
	sort.Sort(util.Int64Slice(ids))
	for _, id := range ids {
		out = append(out, am.activeAlerts[id])
	}
	return json.NewEncoder(w).Encode(out)
}

// AddAlert inserts the given Alert into the AlertManager, if one does not
// already exist for its rule, and fires its actions if inserted.
func (am *AlertManager) AddAlert(a *Alert) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	var alert *Alert
	active := am.activeAlert(a.Name)
	t := time.Now().UTC().Unix()
	if active != 0 {
		// If the alert is already active, just update LastFired.
		alert = am.activeAlerts[active]
		alert.LastFired = t
	} else {
		// Otherwise, insert a new alert.
		alert = a
		// Force some initial values.
		a.Id = 0
		a.Triggered = t
		a.SnoozedUntil = 0
		a.DismissedAt = 0
		a.LastFired = t
		a.Comments = []*Comment{}

		// Add a PrintAction if there isn't one already.
		found := false
		for _, action := range a.Actions {
			if reflect.TypeOf(action).String() == reflect.TypeOf(NewPrintAction()).String() {
				found = true
				break
			}
		}
		if !found {
			a.Actions = append(a.Actions, NewPrintAction())
		}
	}
	// Insert the alert.
	if err := am.updateAlert(alert); err != nil {
		return fmt.Errorf("Failed to add Alert: %v", err)
	}

	// Trigger the alert actions if we inserted a new alert.
	if active == 0 {
		for _, action := range alert.Actions {
			go action.Fire(alert)
		}
	}
	return nil
}

// activeAlert returns the ID for the active alert with the given name, or
// zero if no alert with the given name is active.
func (am *AlertManager) activeAlert(name string) int64 {
	for _, a := range am.activeAlerts {
		if a.Name == name {
			return a.Id
		}
	}
	return 0
}

// ActiveAlert returns the ID for the active alert with the given name, or
// zero if no alert with the given name is active.
func (am *AlertManager) ActiveAlert(name string) int64 {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	return am.activeAlert(name)
}

// updateAlert updates the given Alert in the database and reloads the active
// alerts from the database. Assumes the caller holds a write lock.
func (am *AlertManager) updateAlert(a *Alert) error {
	if err := a.retryReplaceIntoDB(); err != nil {
		return err
	}
	return am.reloadAlerts()
}

// reloadAlerts reloads the active alerts from the database. Assumes the caller
// holds a write lock.
func (am *AlertManager) reloadAlerts() error {
	activeAlerts, err := GetActiveAlerts()
	if err != nil {
		return err
	}
	am.activeAlerts = map[int64]*Alert{}
	for _, a := range activeAlerts {
		am.activeAlerts[a.Id] = a
	}
	return nil
}

// Contains indicates whether this AlertManager has an Alert with the given ID.
func (am *AlertManager) Contains(id int64) bool {
	am.mutex.RLock()
	defer am.mutex.RUnlock()
	_, contains := am.activeAlerts[id]
	return contains
}

// Snooze the given alert until the given time.
func (am *AlertManager) Snooze(id int64, until time.Time, user, message string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	a, ok := am.activeAlerts[id]
	if !ok {
		return fmt.Errorf("Unknown alert: %d", id)
	}
	a.SnoozedUntil = until.UTC().Unix()
	msg := fmt.Sprintf("Snoozed until %s", until.UTC().String())
	if message != "" {
		msg += ": " + message
	}
	return am.addComment(a, &Comment{
		Time:    time.Now().UTC().Unix(),
		User:    user,
		Message: msg,
	})
}

// Unsnooze the given alert.
func (am *AlertManager) Unsnooze(id int64, user, message string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	a, ok := am.activeAlerts[id]
	if !ok {
		return fmt.Errorf("Unknown alert: %d", id)
	}
	a.SnoozedUntil = 0
	msg := fmt.Sprintf("Unsnoozed.")
	if message != "" {
		msg = fmt.Sprintf("Unsnoozed: %s", message)
	}
	return am.addComment(a, &Comment{
		Time:    time.Now().UTC().Unix(),
		User:    user,
		Message: msg,
	})
}

// Dismiss the given alert. Assumes the caller holds a write lock.
func (am *AlertManager) dismiss(a *Alert, user, message string) error {
	now := time.Now().UTC().Unix()
	a.DismissedAt = now
	msg := "Dismissed"
	if message != "" {
		msg = fmt.Sprintf("Dismissed: %s", message)
	}
	if err := am.addComment(a, &Comment{
		Time:    now,
		User:    user,
		Message: msg,
	}); err != nil {
		return err
	}
	return am.updateAlert(a)
}

// Dismiss the given alert.
func (am *AlertManager) Dismiss(id int64, user, message string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	a, ok := am.activeAlerts[id]
	if !ok {
		return fmt.Errorf("Unknown alert: %d", id)
	}
	return am.dismiss(a, user, message)
}

// Add a comment to the given alert. Assumes the caller holds a write lock.
func (am *AlertManager) addComment(a *Alert, c *Comment) error {
	a.Comments = append(a.Comments, c)
	for _, action := range a.Actions {
		go action.Followup(a, fmt.Sprintf("%s: %s", c.User, c.Message))
	}
	return am.updateAlert(a)
}

// Add a comment to the given alert.
func (am *AlertManager) AddComment(id int64, user, msg string) error {
	am.mutex.Lock()
	defer am.mutex.Unlock()
	a, ok := am.activeAlerts[id]
	if !ok {
		return fmt.Errorf("Unknown alert: %d", id)
	}
	return am.addComment(a, &Comment{
		Time:    time.Now().UTC().Unix(),
		User:    user,
		Message: msg,
	})
}

// tick is a function which the AlertManager runs periodically to update its
// Alerts.
func (am *AlertManager) tick() error {
	am.mutex.Lock()
	defer am.mutex.Unlock()

	now := time.Now().UTC().Unix()
	for _, a := range am.activeAlerts {
		// Dismiss alerts whose snooze period has expired.
		if a.Snoozed() && a.SnoozedUntil < now {
			if err := am.dismiss(a, "AlertServer", "Snooze period expired."); err != nil {
				return err
			}
		}
		// Dismiss alerts whose auto-dismiss period has expired.
		if a.AutoDismiss != 0 && a.AutoDismiss < int64(time.Since(time.Unix(a.LastFired, 0))) {
			if err := am.dismiss(a, "AlertServer", fmt.Sprintf("Alert has not fired in %s", time.Duration(a.AutoDismiss).String())); err != nil {
				return err
			}
		}
		// Send a nag message, if applicable.
		if !a.Snoozed() && a.Nag != 0 {
			lastMsgTime := a.Triggered
			if len(a.Comments) > 0 {
				lastMsgTime = a.Comments[len(a.Comments)-1].Time
			}
			if time.Since(time.Unix(int64(lastMsgTime), 0)) > time.Duration(a.Nag) {
				if err := am.addComment(a, &Comment{
					Time:    time.Now().UTC().Unix(),
					User:    "AlertServer",
					Message: fmt.Sprintf(NAG_MSG_TMPL, time.Duration(a.Nag).String()),
				}); err != nil {
					return err
				}
			}
		}
	}

	return am.reloadAlerts()
}

// loop runs the AlertManager's main loop.
func (am *AlertManager) loop() {
	if err := am.tick(); err != nil {
		glog.Error(err)
	}
	for _ = range time.Tick(am.tickInterval) {
		select {
		case <-am.interrupt:
			return
		default:
		}
		if err := am.tick(); err != nil {
			glog.Error(err)
		}
	}
}

// Stop causes the AlertManager to stop running.
func (am *AlertManager) Stop() {
	am.interrupt <- true
}

// MakeAlertManager creates and returns an AlertManager instance.
func MakeAlertManager(tickInterval time.Duration, e *email.GMail) (*AlertManager, error) {
	if Manager != nil {
		return nil, fmt.Errorf("An AlertManager instance already exists!")
	}
	emailAuth = e
	Manager = &AlertManager{
		interrupt:    make(chan bool),
		tickInterval: tickInterval,
	}
	go Manager.loop()
	return Manager, nil
}
