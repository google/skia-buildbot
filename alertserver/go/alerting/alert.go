package alerting

import "encoding/json"

const INFRA_ALERT = "infra"

type alertFields struct {
	Id           int64      `json:"id"`
	Name         string     `json:"name"`
	Category     string     `json:"category"`
	Triggered    int64      `json:"triggered"`
	SnoozedUntil int64      `json:"snoozedUntil"`
	DismissedAt  int64      `json:"dismissedAt"`
	Message      string     `json:"message"`
	Nag          int64      `json:"nag"`
	AutoDismiss  int64      `json:"autoDismiss"`
	LastFired    int64      `json:"lastFired"`
	Comments     []*Comment `json:"comments"`
	Actions      []string   `json:"actions"`
}

// Alert is an object which represents an active alert.
type Alert struct {
	Id           int64      `db:"id"           json:"id"`
	Name         string     `db:"name"         json:"name"`
	Category     string     `db:"category"     json:"category"`
	Triggered    int64      `db:"triggered"    json:"triggered"`
	SnoozedUntil int64      `db:"snoozedUntil" json:"snoozedUntil"`
	DismissedAt  int64      `db:"dismissedAt"  json:"dismissedAt"`
	Message      string     `db:"message"      json:"message"`
	Nag          int64      `db:"nag"          json:"nag"`
	AutoDismiss  int64      `db:"autoDismiss"  json:"autoDismiss"`
	LastFired    int64      `db:"lastFired"    json:"lastFired"`
	Comments     []*Comment `db:"-"            json:"comments"`
	Actions      []Action   `db:"-"            json:"-"`
}

func (a *Alert) MarshalJSON() ([]byte, error) {
	actions := make([]string, 0, len(a.Actions))
	for _, action := range a.Actions {
		actions = append(actions, action.String())
	}
	fields := alertFields{
		Id:           a.Id,
		Name:         a.Name,
		Category:     a.Category,
		Triggered:    a.Triggered,
		SnoozedUntil: a.SnoozedUntil,
		DismissedAt:  a.DismissedAt,
		Message:      a.Message,
		Nag:          a.Nag,
		AutoDismiss:  a.AutoDismiss,
		LastFired:    a.LastFired,
		Comments:     a.Comments,
		Actions:      actions,
	}
	if fields.Comments == nil {
		fields.Comments = []*Comment{}
	}
	return json.Marshal(fields)
}

func (a *Alert) UnmarshalJSON(b []byte) error {
	var proxy alertFields
	if err := json.Unmarshal(b, &proxy); err != nil {
		return err
	}
	a.Id = proxy.Id
	a.Name = proxy.Name
	a.Category = proxy.Category
	a.Triggered = proxy.Triggered
	a.SnoozedUntil = proxy.SnoozedUntil
	a.DismissedAt = proxy.DismissedAt
	a.Message = proxy.Message
	a.Nag = proxy.Nag
	a.AutoDismiss = proxy.AutoDismiss
	a.LastFired = proxy.LastFired
	a.Comments = proxy.Comments
	actions := make([]Action, 0, len(proxy.Actions))
	for _, s := range proxy.Actions {
		action, err := ParseAction(s)
		if err != nil {
			return err
		}
		actions = append(actions, action)
	}
	a.Actions = actions
	return nil
}

// Comment is an object representing a comment on an alert.
type Comment struct {
	User    string `db:"user"    json:"user"`
	Time    int64  `db:"time"    json:"time"`
	Message string `db:"message" json:"message"`
}

// Snoozed indicates whether the Alert has been Snoozed.
func (a *Alert) Snoozed() bool {
	return a.SnoozedUntil != 0
}
