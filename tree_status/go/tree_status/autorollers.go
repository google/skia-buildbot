package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/tree_status/go/types"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// Autoroller - describes an autoroller to query autoroll_status with.
// Eg: ID: "skia-autoroll", Host: "autoroll.skia.org".
type Autoroller struct {
	ID   string
	Host string
}

var (
	// nameToAutoroller is a map of autoroller display names to their ID and hosts.
	nameToAutoroller = map[string]*Autoroller{
		"ANGLE":       {ID: "angle-skia-autoroll", Host: "autoroll.skia.org"},
		"Android":     {ID: "android-master-autoroll", Host: "skia-autoroll.corp.goog"},
		"Chrome":      {ID: "skia-autoroll", Host: "autoroll.skia.org"},
		"Dawn":        {ID: "dawn-skia-autoroll", Host: "autoroll.skia.org"},
		"Google3":     {ID: "google3-autoroll", Host: "skia-autoroll.corp.goog"},
		"Flutter":     {ID: "skia-flutter-autoroll", Host: "autoroll.skia.org"},
		"Skcms":       {ID: "skcms-skia-autoroll", Host: "autoroll.skia.org"},
		"SwiftShader": {ID: "swiftshader-skia-autoroll", Host: "autoroll.skia.org"},
		"VulkanDeps":  {ID: "vulkan-deps-skia-autoroll", Host: "autoroll.skia.org"},
	}

	// Channel that will determine which rollers need to be watched.
	// Entries will look like this: "Android, Chrome, Flutter".
	rollersToWatch = make(chan string, 1)
)

func getAutorollersSnapshot(ctx context.Context, db status.DB) ([]*types.AutorollerSnapshot, error) {
	autorollersSnapshot := []*types.AutorollerSnapshot{}
	if db == nil {
		// Return an empty slice if there is no db specified.
		return autorollersSnapshot, nil
	}
	for name, autoroller := range nameToAutoroller {
		s, err := db.Get(ctx, autoroller.ID)
		if err != nil {
			return nil, fmt.Errorf("Could not get the status of %s: %s", autoroller.ID, err)
		}
		snapshot := &types.AutorollerSnapshot{
			DisplayName: name,
			NumFailed:   s.AutoRollMiniStatus.NumFailedRolls,
			Url:         fmt.Sprintf("https://%s/r/%s", autoroller.Host, autoroller.ID),
		}
		autorollersSnapshot = append(autorollersSnapshot, snapshot)
	}
	sort.Slice(autorollersSnapshot, func(i, j int) bool {
		return autorollersSnapshot[i].DisplayName < autorollersSnapshot[j].DisplayName
	})
	return autorollersSnapshot, nil
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

func AutorollersInit(ctx context.Context, repo string, ts oauth2.TokenSource) (status.DB, error) {
	if err := ds.InitWithOpt(common.PROJECT_ID, ds.AUTOROLL_NS, option.WithTokenSource(ts)); err != nil {
		return nil, skerr.Wrapf(err, "Failed to initialize Cloud Datastore for autorollers")
	}
	db := status.NewDatastoreDB()

	// Start goroutine to watch for rollers.
	go func() {
		for {
			rollers := <-rollersToWatch
			sklog.Infof("Checking for rollers: %s", rollers)
			if rollers == "" {
				continue
			}
			rollsLanded := true
			rollerNames := strings.Split(rollers, ", ")
			for _, rollerName := range rollerNames {
				roller := nameToAutoroller[rollerName]
				s, err := db.Get(ctx, roller.ID)
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
				if err := AddStatus(repo, message, "tree-status@skia.org", types.OpenState, ""); err != nil {
					sklog.Infof("Failed to add automated message to the datastore: %s", err)
				}
			} else {
				rollersToWatch <- rollers
				sklog.Info("Sleeping for 10 seconds")
				time.Sleep(10 * time.Second)
			}
		}
	}()

	return db, nil
}

// HTTP Handlers.

func (srv *Server) autorollersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	as, err := getAutorollersSnapshot(r.Context(), srv.autorollDB)
	if err != nil {
		httputils.ReportError(w, err, "Failed to get autoroll statuses.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(as); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}
