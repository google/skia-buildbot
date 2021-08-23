package common

// Revision represents repo HEAD info for storage as json.
type Revision struct {
	Hash string `json:"hash"`
	URL  string `json:"url"`
}

// Metadata represents repo metadata and list of demos, for storage as json.
type Metadata struct {
	Rev Revision `json:"revision"`
	// In the future we may include actual author information etc, but for now we just list the
	// available demos.
	DemoList []string `json:"demos"`
}
