package autoroll

import (
	"fmt"
	"regexp"
	"strings"
)

import (
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/rietveld"
)

const (
	RIETVELD_URL     string = "http://codereview.chromium.org"
	AUTO_ROLL_AUTHOR string = "skia-deps-roller@chromium.org"
	STATUS_IDLE      string = "Idle"
	STATUS_ACTIVE    string = "Active"
	STATUS_STOPPED   string = "Stopped"
)

// CurrentRoll returns a rietveld.Issue corresponding to the current roll, or
// nil if no roll is in progress.
func CurrentRoll() (*rietveld.Issue, error) {
	open := true
	inProgress, err := rietveld.New(RIETVELD_URL).Search(
		rietveld.SearchOwner(AUTO_ROLL_AUTHOR),
		rietveld.SearchOpen(open),
	)
	if err != nil {
		return nil, fmt.Errorf("Unable to obtain current roll: %v", err)
	}
	if len(inProgress) == 0 {
		return nil, nil
	}
	return inProgress[0], nil
}

// rollRevision returns the commit hash to which the given Issue rolls.
func rollRevision(roll *rietveld.Issue, skiaRepo *gitinfo.GitInfo) (string, error) {
	re := regexp.MustCompile("Roll .* [0-9a-f]+:([0-9a-f]+)")
	badFmtMsg := fmt.Errorf("DEPS roll CL subject line doesn't match expected format (%d)", roll.Issue)
	match := re.FindAllStringSubmatch(roll.Subject, -1)
	if match == nil {
		return "", badFmtMsg
	}
	if len(match) != 1 {
		return "", badFmtMsg
	}
	if len(match[0]) != 2 {
		return "", badFmtMsg
	}
	rev, err := skiaRepo.FullHash(match[0][1])
	if err != nil {
		return "", fmt.Errorf("Could not find revision %s: %v", match[0][1], err)
	}
	return rev, nil
}

// LastRollRevision returns the commit hash for the last-completed roll.
func LastRollRevision(skiaRepo, chromiumRepo *gitinfo.GitInfo) (string, error) {
	deps, err := chromiumRepo.GetFile("DEPS", "origin/master")
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile("'skia_revision': '([0-9a-f]+)',")
	match := re.FindAllStringSubmatch(deps, -1)
	err = fmt.Errorf("Failed to parse the skia_revision from the DEPS file: \n%s", deps)
	if match == nil {
		return "", err
	}
	if len(match) != 1 {
		return "", err
	}
	if len(match[0]) != 2 {
		return "", err
	}
	return match[0][1], nil
}

// AutoRollStatus is a summary of the current status of the DEPS roller.
type AutoRollStatus struct {
	LastRollRevision    string `json:"lastRollRevision"    influxdb:"last_roll_revision"`
	CurrentRollRevision string `json:"currentRollRevision" influxdb:"current_roll_revision"`
	CurrentRoll         int    `json:"currentRoll"         influxdb:"current_roll"`
	Head                string `json:"head"                influxdb:"head"`
	Status              string `json:"status"              influxdb:"status"`
}

// CurrentStatus returns an up-to-date AutoRollStatus object.
func CurrentStatus(skiaRepo, chromiumRepo *gitinfo.GitInfo) (*AutoRollStatus, error) {
	currentRoll, err := CurrentRoll()
	if err != nil {
		return nil, fmt.Errorf("Could not obtain current AutoRoll status: %v", err)
	}
	currentRollRev := "None"
	status := STATUS_IDLE
	issue := 0
	if currentRoll != nil {
		issue = currentRoll.Issue
		currentRollRev, err = rollRevision(currentRoll, skiaRepo)
		if err != nil {
			return nil, err
		}
		status = STATUS_ACTIVE
		for _, m := range currentRoll.Messages {
			if strings.Contains(m.Text, "STOP") {
				status = STATUS_STOPPED
				break
			}
		}
	}
	head, err := skiaRepo.FullHash("origin/master")
	if err != nil {
		return nil, fmt.Errorf("Unable to determine current AutoRoll status: %v", err)
	}
	lastRev, err := LastRollRevision(skiaRepo, chromiumRepo)
	if err != nil {
		return nil, fmt.Errorf("Unable to determine last roll revision: %v", err)
	}
	return &AutoRollStatus{
		LastRollRevision:    lastRev,
		CurrentRollRevision: currentRollRev,
		CurrentRoll:         issue,
		Head:                head,
		Status:              status,
	}, nil
}
