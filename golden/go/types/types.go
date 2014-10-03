package types

// Label for classifying digests.
type Label int

const (
	// Primary key field that uniquely identifies a key.
	PRIMARY_KEY_FIELD = "name"

	// Classifications for observed digests.
	UNTRIAGED Label = iota
	POSITIVE
	NEGATIVE
)

// Stores the digests and their associated labels.
// Note: The name of the test is assumed to be handled by the client of this
// type. Most likely in the keys of a map.
type TestClassification map[string]Label

func (tc TestClassification) DeepCopy() TestClassification {
	result := make(map[string]Label, len(tc))
	for k, v := range tc {
		result[k] = v
	}
	return result
}
