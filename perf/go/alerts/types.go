package alerts

// AlertStore is the interface used to persist Alerts.
type AlertStore interface {
	Save(cfg *Alert) error
	Delete(id int) error
	List(includeDeleted bool) ([]*Alert, error)
}
