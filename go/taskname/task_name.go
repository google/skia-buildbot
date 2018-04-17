package taskname

// schema is a sub-struct of JobNameSchema.
type SchemaDetails struct {
	Keys         []string `json:"keys"`
	OptionalKeys []string `json:"optional_keys"`
	RecurseRoles []string `json:"recurse_roles"`
}

// JobNameSchema is a struct used for (de)constructing Job names in a
// predictable format.
type JobNameSchema struct {
	Schema map[string]*SchemaDetails `json:"builder_name_schema"`
	Sep    string                    `json:"builder_name_sep"`
}
