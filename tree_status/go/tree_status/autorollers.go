package main

import (
	"context"
	"fmt"
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

	AutorollStatuses = []autoRollStatus{}
	NamesToRollers   = map[string]string{}
	// Channel that will determine which rollers need to be watched.
	RollersToWatch chan string
)

type autoRollStatus struct {
	Name      string `json:"name"`
	NumFailed int    `json:"num_failed"`
	Url       string `json:"url"`
}

func AutorollersInit(ctx context.Context, ts oauth2.TokenSource) error {
	if err := ds.InitWithOpt(common.PROJECT_ID, ds.AUTOROLL_NS, option.WithTokenSource(ts)); err != nil {
		return skerr.Wrapf(err, "Failed to initialize Cloud Datastore for autorollers")
	}

	// Populate the AutorollStatuses and NamesToRollers objects.
	for host, subMap := range AUTOROLLERS {
		for roller, friendlyName := range subMap {
			s, err := autoroll_status.Get(ctx, roller)
			if err != nil {
				return fmt.Errorf("Could not get the status of %s: %s", roller, err)
			}
			status := autoRollStatus{
				Name:      friendlyName,
				NumFailed: s.AutoRollMiniStatus.NumFailedRolls,
				Url:       fmt.Sprintf("https://%s/r/%s", host, roller),
			}
			AutorollStatuses = append(AutorollStatuses, status)
			NamesToRollers[friendlyName] = roller
		}
	}
	RollersToWatch = make(chan string, len(AutorollStatuses))

	// Start goroutine to watch for rollers.
	go func() {
		for {
			select {
			case roller := <-RollersToWatch:
				if roller == "" {
					continue
				}

				fmt.Printf("Looking for status of %s\n", roller)
				s, err := autoroll_status.Get(ctx, roller)
				if err != nil {
					sklog.Errorf("Could not get status of %s: %s", roller, err)
					// Continue so that we can try again.
					RollersToWatch <- roller
					continue
				}
				if s.AutoRollMiniStatus.NumFailedRolls == 0 {
					sklog.Infof("Roller %s has 0 NumFailedRolls", roller)
					fmt.Println("Going to move on from this roller now")
					continue
				}
				sklog.Infof("Roller %s still has %d NumFailedRolls", roller, s.AutoRollMiniStatus.NumFailedRolls)
				RollersToWatch <- roller
				fmt.Println("Sleeping for 10 seconds")
				time.Sleep(10 * time.Second)
			}
			fmt.Println("OUT OF THE LOOP.")
		}
	}()

	return nil
}

func StartWatchingAutorollers(rollers string) {
	rollerNames := strings.Split(rollers, ",")
	for _, r := range rollerNames {
		RollersToWatch <- NamesToRollers[r]
	}
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
