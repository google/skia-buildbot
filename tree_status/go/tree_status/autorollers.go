package main

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	autoroll_status "go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skerr"
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

	AutorollStatuses []autoRollStatus
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

	AutorollStatuses = []autoRollStatus{}
	updateAutorollStatus := func(ctx context.Context) error {
		AutorollStatuses = []autoRollStatus{}
		for host, subMap := range AUTOROLLERS {
			for roller, friendlyName := range subMap {
				s, err := autoroll_status.Get(ctx, roller)
				if err != nil {
					fmt.Println("XXXXXXXXXXXXXXXXXX")
					fmt.Println(err)
					fmt.Println(roller)
					fmt.Println(friendlyName)
					return err
				}
				status := autoRollStatus{
					Name:      friendlyName,
					NumFailed: s.AutoRollMiniStatus.NumFailedRolls,
					Url:       fmt.Sprintf("https://%s/r/%s", host, roller),
				}
				AutorollStatuses = append(AutorollStatuses, status)
			}
		}
		return nil
	}
	if err := updateAutorollStatus(ctx); err != nil {
		return err
	}
	fmt.Println("GOT THISSSS")
	fmt.Printf("%+v", AutorollStatuses)
	return nil
}
