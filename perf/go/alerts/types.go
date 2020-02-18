package alerts

// AlertStore is the interface used to persist Alerts.
type AlertStore interface {
	// Save can write a new, or update an existing, Config. New Config's will
	// have an ID of -1.
	Save(cfg *Alert) error

	// Delete removes the Alert with the given id.
	Delete(id int) error

	// List retrieves all the Alerts.
	//
	// If includeDeleted is true then deleted Alerts are also included in the
	// response.
	List(includeDeleted bool) ([]*Alert, error)
}
