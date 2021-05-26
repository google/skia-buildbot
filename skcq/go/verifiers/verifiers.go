package verifiers

import (
	"net/http"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/skcq/go/config"
)

type Verifier interface {
	// Name of the verifier.
	Name() string

	// Run the verifier. It returns a bool that denotes whether verification was successful.
	// If verification was not successful then it also returns a reason.
	Verify(cfg *config.SkCQCfg) (bool, string, error)
}

func NewCommitterListVerifier(httpClient *http.Client, criaGroup string) (*Verifier, error) {
	committerAllowed, err := allowed.NewAllowedFromChromeInfraAuth(httpClient, criaGroup)
	if err != nil {
		return nil, skerr.Fmt("Could not create an allowed from %s: %s", criaGroup, err)
	}
	return &CommitterListVerifier{
		criaGroupName: criaGroup,
		committerAllowed: committerAllowed,
	}, nil
}

type CommitterListVerifier struct {
	criaGroupName string
	committerAllowed *allowed.AllowedFromChromeInfraAuth
}

func (cv *CommitterListVerifier) Name() string {
	return "[CommitterListVerifier]"
}

func (cv *CommitterListVerifier) Verify(cfg *config.SkCQCfg) (bool, string, error) {
	if (cv.committerAllowed.Member(cfg.CommitterList)) {
		return true, "", nil
	} else {
		return false, fmt.Sprintf("%s is not a member of the cria group %s", c)
	}
}
