package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	autoroll_status "go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
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

	NamesToRollers = map[string]string{}
	// Channel that will determine which rollers need to be watched.
	RollersToWatch = make(chan string, 1)
)

// RENAME TO AUTOROLL_DETAIL OR INFO OR SOMETHING ELSE!
type AutorollStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	NumFailed int    `json:"num_failed"`
	Url       string `json:"url"`
}

func GetAutorollStatuses(ctx context.Context) ([]*AutorollStatus, error) {
	autorollStatuses := []*AutorollStatus{}
	for host, subMap := range AUTOROLLERS {
		for roller, friendlyName := range subMap {
			s, err := autoroll_status.Get(ctx, roller)
			if err != nil {
				return nil, fmt.Errorf("Could not get the status of %s: %s", roller, err)
			}
			status := &AutorollStatus{
				ID:        roller,
				Name:      friendlyName,
				NumFailed: s.AutoRollMiniStatus.NumFailedRolls,
				Url:       fmt.Sprintf("https://%s/r/%s", host, roller),
			}
			autorollStatuses = append(autorollStatuses, status)
		}
	}
	sort.Slice(autorollStatuses, func(i, j int) bool {
		return autorollStatuses[i].Name < autorollStatuses[j].Name
	})
	return autorollStatuses, nil
}

func AutorollersInit(ctx context.Context, ts oauth2.TokenSource) error {
	if err := ds.InitWithOpt(common.PROJECT_ID, ds.AUTOROLL_NS, option.WithTokenSource(ts)); err != nil {
		return skerr.Wrapf(err, "Failed to initialize Cloud Datastore for autorollers")
	}

	autorollStatus, err := GetAutorollStatuses(ctx)
	if err != nil {
		return err
	}
	for _, as := range autorollStatus {
		NamesToRollers[as.Name] = as.ID
	}

	// Start goroutine to watch for rollers.
	go func() {
		for {
			select {
			case rollers := <-RollersToWatch:
				sklog.Infof("Checking for rollers: %s", rollers)
				if rollers == "" {
					continue
				}
				rollsLanded := true
				rollerNames := strings.Split(rollers, ",")
				for _, rollerName := range rollerNames {
					roller := NamesToRollers[rollerName]
					s, err := autoroll_status.Get(ctx, roller)
					if err != nil {
						sklog.Errorf("Could not get status of %s: %s", roller, err)
						// Continue so that we can try again.
						rollsLanded = false
						continue
					}
					if s.AutoRollMiniStatus.NumFailedRolls == 0 {
						sklog.Infof("Roller %s has 0 NumFailedRolls", roller)
						rollsLanded = rollsLanded && true
						continue
					} else {
						sklog.Infof("Roller %s has %d NumFailedRolls. Continue the loop.", roller, s.AutoRollMiniStatus.NumFailedRolls)
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
					if err := AddStatus(message, "tree-status@skia.org", ""); err != nil {
						sklog.Infof("Failed to add automated message to the datastore: %s", err)
					}
				} else {
					RollersToWatch <- rollers
					sklog.Info("Sleeping for 10 seconds")
					time.Sleep(10 * time.Second)
				}
			}
		}
	}()

	return nil
}

func StartWatchingAutorollers(rollers string) {
	RollersToWatch <- rollers
}

func StopWatchingAutorollers() {
	// Empty the RollersToWatch channel.
	fmt.Println("EMPTYING THE CHANNEL NOW!")
L:
	for {
		select {
		case <-RollersToWatch:
		default:
			break L
		}
	}
	fmt.Println("DONE EMPTYING THE CHANNEL!")
}
