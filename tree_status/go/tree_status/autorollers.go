package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	autoroll_status "go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

var (
	// AUTOROLLERS maps autoroll frontend host to maps of roller IDs to
	// their human-friendly display names.
	AUTOROLLERS = map[string]map[string]string{
		"autoroll.skia.org": map[string]string{
			"skia-flutter-autoroll":     "Flutter",
			"skia-autoroll":             "Chrome",
			"angle-skia-autoroll":       "ANGLE",
			"skcms-skia-autoroll":       "Skcms",
			"swiftshader-skia-autoroll": "SwiftShader",
		},
		"skia-autoroll.corp.goog": map[string]string{
			"android-master-autoroll": "Android",
			"google3-autoroll":        "Google3",
		},
	}

	namesToRollers = map[string]string{}
	// Channel that will determine which rollers need to be watched.
	rollersToWatch = make(chan string, 1)
)

// RENAME TO AUTOROLL_DETAIL OR INFO OR SOMETHING ELSE!
type AutorollerSnapshot struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	NumFailed int    `json:"num_failed"`
	Url       string `json:"url"`
}

func getAutorollersSnapshot(ctx context.Context) ([]*AutorollerSnapshot, error) {
	autorollersSnapshot := []*AutorollerSnapshot{}
	for host, subMap := range AUTOROLLERS {
		for roller, friendlyName := range subMap {
			s, err := autoroll_status.Get(ctx, roller)
			if err != nil {
				return nil, fmt.Errorf("Could not get the status of %s: %s", roller, err)
			}
			snapshot := &AutorollerSnapshot{
				ID:        roller,
				Name:      friendlyName,
				NumFailed: s.AutoRollMiniStatus.NumFailedRolls,
				Url:       fmt.Sprintf("https://%s/r/%s", host, roller),
			}
			autorollersSnapshot = append(autorollersSnapshot, snapshot)
		}
	}
	sort.Slice(autorollersSnapshot, func(i, j int) bool {
		return autorollersSnapshot[i].Name < autorollersSnapshot[j].Name
	})
	return autorollersSnapshot, nil
}

func AutorollersInit(ctx context.Context, ts oauth2.TokenSource) error {
	if err := ds.InitWithOpt(common.PROJECT_ID, ds.AUTOROLL_NS, option.WithTokenSource(ts)); err != nil {
		return skerr.Wrapf(err, "Failed to initialize Cloud Datastore for autorollers")
	}

	autorollersSnapshot, err := getAutorollersSnapshot(ctx)
	if err != nil {
		return err
	}
	for _, as := range autorollersSnapshot {
		namesToRollers[as.Name] = as.ID
	}

	// Start goroutine to watch for rollers.
	go func() {
		for {
			select {
			case rollers := <-rollersToWatch:
				sklog.Infof("Checking for rollers: %s", rollers)
				if rollers == "" {
					continue
				}
				rollsLanded := true
				rollerNames := strings.Split(rollers, ", ")
				for _, rollerName := range rollerNames {
					roller := namesToRollers[rollerName]
					s, err := autoroll_status.Get(ctx, roller)
					if err != nil {
						sklog.Errorf("Could not get status of %s: %s\n", roller, err)
						// Continue so that we can try again.
						rollsLanded = false
						continue
					}
					if s.AutoRollMiniStatus.NumFailedRolls == 0 {
						sklog.Infof("Roller %s has 0 NumFailedRolls\n", roller)
						rollsLanded = rollsLanded && true
						continue
					} else {
						sklog.Infof("Roller %s has %d NumFailedRolls. Continue the loop.\n", roller, s.AutoRollMiniStatus.NumFailedRolls)
						rollsLanded = false
						continue
					}
				}
				if rollsLanded {
					// Send status notification.
					rollerText := "roller"
					if len(rollerNames) > 1 {
						rollerText = fmt.Sprintf("%ss", rollerText)
					}
					message := fmt.Sprintf("Open: %s %s landed", rollers, rollerText)
					sklog.Infof("Sending status notification with message: \"%s\"", message)
					if err := AddStatus(message, "tree-status@skia.org", OPEN_STATE, ""); err != nil {
						sklog.Infof("Failed to add automated message to the datastore: %s", err)
					}
				} else {
					rollersToWatch <- rollers
					sklog.Info("Sleeping for 10 seconds")
					time.Sleep(10 * time.Second)
				}
			}
		}
	}()

	return nil
}

func StartWatchingAutorollers(rollers string) {
	rollersToWatch <- rollers
}

func StopWatchingAutorollers() {
	// Empty the RollersToWatch channel.
L:
	for {
		select {
		case <-rollersToWatch:
		default:
			break L
		}
	}
}

// HTTP Handlers.

func (srv *Server) autorollersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	as, err := getAutorollersSnapshot(r.Context())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get autoroll statuses.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(as); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}
