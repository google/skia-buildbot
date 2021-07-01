package verifiers

import (
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/gerrit"
)

// GetAllowedVoters is a utility function that looks through the labels on a gerrit change to gather the
// email addresses of voters who voted the specified labelValue and who are in the allowedCRIA group.
func GetAllowedVoters(ci *gerrit.ChangeInfo, allowedCRIA *allowed.AllowedFromChromeInfraAuth, labelName string, labelValue int) []string {
	allowedVoters := []string{}
	if val, ok := ci.Labels[labelName]; ok {
		for _, ld := range val.All {
			if ld.Value == labelValue {
				if allowedCRIA.Member(ld.Email) {
					allowedVoters = append(allowedVoters, ld.Email)
				}
			}
		}
	}
	return allowedVoters
}
