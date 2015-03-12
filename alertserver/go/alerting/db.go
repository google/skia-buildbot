package alerting

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/util"
)

// commentFromDB is a convenience struct which handles nullable database fields.
type commentFromDB struct {
	Id      int64  `db:"id"`
	AlertId int64  `db:"alertId"`
	Time    int64  `db:"time"`
	User    string `db:"user"`
	Message string `db:"message"`
}

// toComment converts a commentFromDB to a Comment.
func (c commentFromDB) toComment() *Comment {
	return &Comment{
		Time:    c.Time,
		User:    c.User,
		Message: c.Message,
	}
}

// actionFromDB is a convenience struct which handles nullable database fields.
type actionFromDB struct {
	Id      int64  `db:"id"`
	AlertId int64  `db:"alertId"`
	Action  string `db:"action"`
}

// toAction converts an actionFromDB to an Action.
func (a actionFromDB) toAction() (Action, error) {
	return ParseAction(a.Action)
}

// GetActiveAlerts retrieves all active alerts.
func GetActiveAlerts() ([]*Alert, error) {
	// Get the Alerts.
	rv := []*Alert{}
	if err := DB.Select(&rv, fmt.Sprintf("SELECT id,name,category,triggered,snoozedUntil,dismissedAt,message,nag FROM %s WHERE active = 1;", TABLE_ALERTS)); err != nil {
		return nil, fmt.Errorf("Could not retrieve active alerts: %v", err)
	}

	if len(rv) == 0 {
		return []*Alert{}, nil
	}

	interfaceIds := make([]interface{}, 0, len(rv))
	alertsById := map[int64]*Alert{}
	for _, a := range rv {
		interfaceIds = append(interfaceIds, a.Id)
		alertsById[a.Id] = a
	}
	inputTmpl := util.RepeatJoin("?", ",", len(interfaceIds))

	// Get the Comments.
	comments := []*commentFromDB{}
	if err := DB.Select(&comments, fmt.Sprintf("SELECT * FROM %s WHERE alertId IN (%s);", TABLE_COMMENTS, inputTmpl), interfaceIds...); err != nil {
		return nil, fmt.Errorf("Could not retrieve comments for active alerts: %v", err)
	}
	for _, c := range comments {
		alertsById[c.AlertId].Comments = append(alertsById[c.AlertId].Comments, c.toComment())
	}

	// Get the Actions.
	actions := []actionFromDB{}
	if err := DB.Select(&actions, fmt.Sprintf("SELECT * FROM %s WHERE alertId IN (%s);", TABLE_ACTIONS, inputTmpl), interfaceIds...); err != nil {
		return nil, fmt.Errorf("Could not retrieve actions for active alerts: %v", err)
	}
	for _, a := range actions {
		action, err := a.toAction()
		if err != nil {
			return nil, fmt.Errorf("Could not retrieve actions for active alerts: Failed to parse Action: %v", err)
		}
		alertsById[a.AlertId].Actions = append(alertsById[a.AlertId].Actions, action)
	}

	return rv, nil
}

// retryReplaceIntoDB inserts or updates the Alert in the database, making multiple attempts if necessary.
func (a *Alert) retryReplaceIntoDB() error {
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = a.replaceIntoDB(); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// replaceIntoDB inserts or updates the Alert in the database.
func (a *Alert) replaceIntoDB() (rv error) {
	tx, err := DB.Beginx()
	if err != nil {
		return fmt.Errorf("Unable to push Alert into database - Could not begin transaction: %v", err)
	}
	defer func() { rv = database.CommitOrRollback(tx, rv) }()

	// Insert the alert itself.
	active := 0
	if a.DismissedAt == 0 {
		active = 1
	}
	res, err := tx.Exec(fmt.Sprintf("REPLACE INTO %s (id,active,name,triggered,category,message,nag,snoozedUntil,dismissedAt) VALUES (?,?,?,?,?,?,?,?,?);", TABLE_ALERTS), a.Id, active, a.Name, a.Triggered, a.Category, a.Message, a.Nag, a.SnoozedUntil, a.DismissedAt)
	if err != nil {
		return fmt.Errorf("Failed to push alert into database: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("Failed to push alert into database; LastInsertId failed: %v", err)
	}
	a.Id = id

	// Comments.

	// First, delete existing comments so we don't have leftovers hanging around from before.
	if _, err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE alertId = ?;", TABLE_COMMENTS), a.Id); err != nil {
		return fmt.Errorf("Failed to delete comments from database: %v", err)
	}
	// Actually insert the comments.
	if len(a.Comments) > 0 {
		commentFields := 4
		commentTmpl := util.RepeatJoin("?", ",", commentFields)
		commentsTmpl := util.RepeatJoin(fmt.Sprintf("(%s)", commentTmpl), ",", len(a.Comments))
		flattenedComments := make([]interface{}, 0, commentFields*len(a.Comments))
		for _, c := range a.Comments {
			flattenedComments = append(flattenedComments, a.Id, c.User, c.Time, c.Message)
		}
		if _, err := tx.Exec(fmt.Sprintf("INSERT INTO %s (alertId,user,time,message) VALUES %s;", TABLE_COMMENTS, commentsTmpl), flattenedComments...); err != nil {
			return fmt.Errorf("Unable to push comments into database: %v", err)
		}
	}

	// Actions.

	// First, delete existing actions so we don't have leftovers hanging around from before.
	if _, err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE alertId = ?;", TABLE_ACTIONS), a.Id); err != nil {
		return fmt.Errorf("Failed to delete actions from database: %v", err)
	}
	// Actually insert the actions.
	if len(a.Actions) > 0 {
		actionFields := 2
		actionTmpl := util.RepeatJoin("?", ",", actionFields)
		actionsTmpl := util.RepeatJoin(fmt.Sprintf("(%s)", actionTmpl), ",", len(a.Actions))
		flattenedActions := make([]interface{}, 0, actionFields*len(a.Actions))
		for _, action := range a.Actions {
			flattenedActions = append(flattenedActions, a.Id, action.String())
		}
		if _, err := tx.Exec(fmt.Sprintf("INSERT INTO %s (alertId,action) VALUES %s;", TABLE_ACTIONS, actionsTmpl), flattenedActions...); err != nil {
			return fmt.Errorf("Unable to push actions into database: %v", err)
		}
	}

	// the transaction is committed during the deferred function.
	return nil
}
