package ds_tryjobstore

// newerInterface is an internal interface that allows to define a temporal
// order for a type.
type newerInterface interface {
	MoreRecentThan(right interface{}) bool
}
