package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/unrolled/secure"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

const (
	sheriffs         = "Sheriffs"
	sheriffRotations = "SheriffSchedules"

	robocops         = "Robocops"
	robocopRotations = "RobocopSchedules"

	troopers         = "Troopers"
	trooperRotations = "TrooperSchedules"

	wranglers         = "GpuSheriffs"
	wranglerRotations = "GpuSheriffSchedules"
)

var (
	typeToRotations = map[string]string{
		sheriffs:  sheriffRotations,
		robocops:  robocopRotations,
		troopers:  trooperRotations,
		wranglers: wranglerRotations,
	}
)

type Rotation struct {
	Username      string    `json:"username" datastore:"username"`
	ScheduleStart time.Time `json:"schedule_start" datastore:"schedule_start"`
	ScheduleEnd   time.Time `json:"schedule_end" datastore:"schedule_end"`

	ReadableRange string `json:"readable_range" datastore:"-"`
	CurrentWeek   bool   `json:"current_week" datastore:"-"`

	Key *datastore.Key `json:"-" datastore:"-"`
}

type RotationMember struct {
	Username string `json:"username" datastore:"username"`
}

type RotationsTemplateContext struct {
	// Nonce is the CSP Nonce. Look in webpack.config.js for where the nonce
	// templates are injected.
	Nonce string

	RotationsType string
	RotationsDoc  string
	RotationsImg  string
	Rotations     []*Rotation
}

func getUpcomingRotations(rotations string, from time.Time) ([]*Rotation, error) {
	upcomingRotations := []*Rotation{}
	q := datastore.NewQuery(rotations).Namespace(*namespace).Filter("schedule_end >", from).Order("schedule_end")
	it := dsClient.Run(context.TODO(), q)
	for {
		r := &Rotation{}
		k, err := it.Next(r)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of rotations: %s", err)
		}
		_, startMonth, startDate := r.ScheduleStart.UTC().Date()
		_, endMonth, endDate := r.ScheduleEnd.UTC().Date()
		r.ReadableRange = fmt.Sprintf("%d %s - %d %s", startDate, startMonth, endDate, endMonth)
		r.Key = k
		upcomingRotations = append(upcomingRotations, r)
	}
	if len(upcomingRotations) > 0 {
		upcomingRotations[0].CurrentWeek = true
	}
	return upcomingRotations, nil
}

func getRotationMembers(rotationType string) ([]*RotationMember, error) {
	members := []*RotationMember{}
	q := datastore.NewQuery(rotationType).Namespace(*namespace)
	it := dsClient.Run(context.TODO(), q)
	for {
		r := &RotationMember{}
		_, err := it.Next(r)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of rotation members: %s", err)
		}
		members = append(members, r)
	}
	return members, nil
}

func addRotation(rotationType, username string, scheduleStart, scheduleEnd time.Time) error {
	r := &Rotation{
		Username:      username,
		ScheduleStart: scheduleStart,
		ScheduleEnd:   scheduleEnd,
	}

	key := &datastore.Key{
		Kind:      typeToRotations[rotationType],
		Namespace: *namespace,
	}
	_, err := dsClient.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
		var err error
		if _, err = tx.Put(key, r); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("Failed to add rotation: %s", err)
	}
	return nil
}

// HTTP Handlers.

func (srv *Server) currentSheriffHandler(w http.ResponseWriter, r *http.Request) {
	srv.currentRotationHandler(w, r, sheriffRotations)
}

func (srv *Server) currentWranglerHandler(w http.ResponseWriter, r *http.Request) {
	srv.currentRotationHandler(w, r, wranglerRotations)
}

func (srv *Server) currentTrooperHandler(w http.ResponseWriter, r *http.Request) {
	srv.currentRotationHandler(w, r, trooperRotations)
}

func (srv *Server) currentRobocopHandler(w http.ResponseWriter, r *http.Request) {
	srv.currentRotationHandler(w, r, robocopRotations)
}

func (srv *Server) nextSheriffHandler(w http.ResponseWriter, r *http.Request) {
	srv.nextRotationHandler(w, r, sheriffRotations)
}

func (srv *Server) nextWranglerHandler(w http.ResponseWriter, r *http.Request) {
	srv.nextRotationHandler(w, r, wranglerRotations)
}

func (srv *Server) nextTrooperHandler(w http.ResponseWriter, r *http.Request) {
	srv.nextRotationHandler(w, r, trooperRotations)
}

func (srv *Server) nextRobocopHandler(w http.ResponseWriter, r *http.Request) {
	srv.nextRotationHandler(w, r, robocopRotations)
}

func (srv *Server) updateSheriffRotationsHandler(w http.ResponseWriter, r *http.Request) {
	srv.updateRotationsHandler(w, r, sheriffs, srv.sheriffHandler)
}

func (srv *Server) updateWranglerRotationsHandler(w http.ResponseWriter, r *http.Request) {
	srv.updateRotationsHandler(w, r, wranglers, srv.wranglerHandler)
}

func (srv *Server) updateRobocopRotationsHandler(w http.ResponseWriter, r *http.Request) {
	srv.updateRotationsHandler(w, r, robocops, srv.robocopHandler)
}

func (srv *Server) updateTrooperRotationsHandler(w http.ResponseWriter, r *http.Request) {
	srv.updateRotationsHandler(w, r, troopers, srv.trooperHandler)
}

func (srv *Server) sheriffHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := getUpcomingRotations(sheriffRotations, time.Now().UTC())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get sheriff rotations.", http.StatusInternalServerError)
		return
	}
	templateContext := RotationsTemplateContext{
		Nonce:         secure.CSPNonce(r.Context()),
		RotationsType: "Sheriff",
		RotationsDoc:  "https://skia.org/dev/sheriffing",
		RotationsImg:  "sheriff.jpg",
		Rotations:     rotations,
	}
	if err := srv.commonRotationsHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
}

func (srv *Server) wranglerHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := getUpcomingRotations(wranglerRotations, time.Now().UTC())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get wrangler rotations.", http.StatusInternalServerError)
		return
	}
	templateContext := RotationsTemplateContext{
		Nonce:         secure.CSPNonce(r.Context()),
		RotationsType: "Wrangler",
		RotationsDoc:  "https://skia.org/dev/sheriffing/gpu",
		RotationsImg:  "wrangler.jpg",
		Rotations:     rotations,
	}
	if err := srv.commonRotationsHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
}

func (srv *Server) robocopHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := getUpcomingRotations(robocopRotations, time.Now().UTC())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get robocop rotations.", http.StatusInternalServerError)
		return
	}
	templateContext := RotationsTemplateContext{
		Nonce:         secure.CSPNonce(r.Context()),
		RotationsType: "Android Robocop",
		RotationsDoc:  "https://skia.org/dev/sheriffing/android",
		RotationsImg:  "robocop.jpg",
		Rotations:     rotations,
	}
	if err := srv.commonRotationsHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
}

func (srv *Server) trooperHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := getUpcomingRotations(trooperRotations, time.Now().UTC())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get trooper rotations.", http.StatusInternalServerError)
		return
	}
	templateContext := RotationsTemplateContext{
		Nonce:         secure.CSPNonce(r.Context()),
		RotationsType: "Infra Trooper",
		RotationsDoc:  "https://skia.org/dev/sheriffing/trooper",
		RotationsImg:  "trooper.jpg",
		Rotations:     rotations,
	}
	if err := srv.commonRotationsHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
}

// HTTP Handler helpers.

func (srv *Server) currentRotationHandler(w http.ResponseWriter, r *http.Request, rotations string) {
	w.Header().Set("Content-Type", "application/json")

	upcomingRotations, err := getUpcomingRotations(rotations, time.Now().UTC())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get sheriff rotations.", http.StatusInternalServerError)
		return
	}
	var rotation interface{}
	if len(upcomingRotations) == 0 {
		rotation = map[string]string{}
	} else {
		rotation = upcomingRotations[0]
	}
	if err := json.NewEncoder(w).Encode(rotation); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) nextRotationHandler(w http.ResponseWriter, r *http.Request, rotations string) {
	w.Header().Set("Content-Type", "application/json")

	upcomingRotations, err := getUpcomingRotations(rotations, time.Now().UTC())
	if err != nil {
		httputils.ReportError(w, err, "Failed to get sheriff rotations.", http.StatusInternalServerError)
		return
	}
	var rotation interface{}
	if len(upcomingRotations) < 2 {
		rotation = map[string]string{}
	} else {
		rotation = upcomingRotations[1]
	}
	if err := json.NewEncoder(w).Encode(rotation); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) commonRotationsHandler(w http.ResponseWriter, r *http.Request, templateContext RotationsTemplateContext) error {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "rotations.html", templateContext); err != nil {
		return fmt.Errorf("Failed to expand template: %s", err)
	}
	return nil
}

func (srv *Server) updateRotationsHandler(w http.ResponseWriter, r *http.Request, rotationType string, redirectHandler func(w http.ResponseWriter, r *http.Request)) {
	user := srv.user(r)
	if !srv.admin.Member(user) {
		httputils.ReportError(w, nil, "You do not have access to update rotations.", http.StatusInternalServerError)
		return
	}

	scheduleStart := r.URL.Query().Get("schedule_start")
	weeks := r.URL.Query().Get("weeks")
	if scheduleStart == "" || weeks == "" {
		httputils.ReportError(w, nil, "Must specify schedule_start and weeks parameters. Eg: ?schedule_start=2020-01-31&weeks=5", http.StatusBadRequest)
		return
	}
	from, err := time.Parse(time.RFC3339, fmt.Sprintf("%sT00:00:00Z", scheduleStart))
	if err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("schedule_start must be in format of 2020-01-31 not %s", scheduleStart), http.StatusBadRequest)
		return
	}
	if from.Weekday() != time.Monday {
		httputils.ReportError(w, nil, fmt.Sprintf("schedule_start must be a Monday not %s", from.Weekday()), http.StatusBadRequest)
		return
	}
	weeksInt, err := strconv.Atoi(weeks)
	if err != nil || weeksInt < 1 {
		httputils.ReportError(w, nil, fmt.Sprintf("weeks must be an int>1 not %s", weeks), http.StatusBadRequest)
		return
	}

	members, err := getRotationMembers(rotationType)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to get %s rotation members.", rotationType), http.StatusInternalServerError)
		return
	}

	sklog.Infof("Going to update rotations of %s with %d members\n", rotationType, len(members))
	sklog.Infof("Starting at %s for %d weeks\n", from, weeksInt)

	// Clear out any rotations that exist in the time range we want to populate.
	rotations, err := getUpcomingRotations(typeToRotations[rotationType], from.UTC())
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to get rotations of %s.", rotationType), http.StatusInternalServerError)
		return
	}
	for _, rotation := range rotations {
		sklog.Infof("Going to delete %+v rotation\n", rotation)
		if err := dsClient.Delete(r.Context(), rotation.Key); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Could not delete rotations of %s after %s.", rotationType, from), http.StatusInternalServerError)
			return
		}
	}

	membersIndex := 0
	currScheduleFrom := from
	for week := 1; week <= weeksInt; week++ {
		currScheduleEnd := currScheduleFrom.Add(time.Hour * 24 * 6)
		sklog.Infof("Adding %s for %s to %s\n", members[membersIndex].Username, currScheduleFrom, currScheduleEnd)
		if err := addRotation(rotationType, members[membersIndex].Username, currScheduleFrom, currScheduleEnd); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Could not create new rotations of %s.", rotationType), http.StatusInternalServerError)
			return
		}
		currScheduleFrom = currScheduleFrom.Add(time.Hour * 24 * 7)
		membersIndex = (membersIndex + 1) % len(members)
	}

	redirectHandler(w, r)
}
