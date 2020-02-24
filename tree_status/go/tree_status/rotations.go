package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/unrolled/secure"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
)

var (
	RotationTypeToKind = map[string]string{
		"Sheriffs":    "SheriffSchedules",
		"Robocops":    "RobocopSchedules",
		"Troopers":    "TrooperSchedules",
		"GpuSheriffs": "GpuSheriffSchedules",
	}
)

type Rotation struct {
	Username      string    `json:"username" datastore:"username"`
	ScheduleStart time.Time `json:"schedule_start" datastore:"schedule_start"`
	ScheduleEnd   time.Time `json:"schedule_end" datastore:"schedule_end"`

	ReadableRange string         `json:"readable_range" datastore:"-"`
	CurrentWeek   bool           `json:"current_week" datastore:"-"`
	Key           *datastore.Key `json:"key" datastore:"-"`
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

func GetUpcomingRotations(rotationType string, from time.Time) ([]*Rotation, error) {
	rotations := []*Rotation{}
	q := datastore.NewQuery(rotationType).Namespace("").Filter("schedule_end >", from).Order("schedule_end")
	it := DS.Run(context.TODO(), q)
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
		rotations = append(rotations, r)
	}
	if len(rotations) > 0 {
		rotations[0].CurrentWeek = true
	}
	return rotations, nil
}

func GetRotationMembers(rotationType string) ([]*RotationMember, error) {
	members := []*RotationMember{}
	q := datastore.NewQuery(rotationType).Namespace("")
	it := DS.Run(context.TODO(), q)
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

// HTTP Handlers.

func (srv *Server) updateSheriffRotationsHandler(w http.ResponseWriter, r *http.Request) {
	srv.updateRotationsHandler(w, r, "Sheriffs", srv.sheriffHandler)
}

// TODO(rmistry): Something is confusing with timestamps here and to try to make it behave like it does right now.....

func AddRotation(rotationType, username string, scheduleStart, scheduleEnd time.Time) error {
	r := &Rotation{
		Username:      username,
		ScheduleStart: scheduleStart,
		ScheduleEnd:   scheduleEnd,
	}

	key := &datastore.Key{
		Kind:      RotationTypeToKind[rotationType],
		Namespace: "",
	}
	_, err := DS.RunInTransaction(context.Background(), func(tx *datastore.Transaction) error {
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

// TODO(rmistry): Keep generic handlers here and move the specific ones to main.go
func (srv *Server) updateRotationsHandler(w http.ResponseWriter, r *http.Request, rotationType string, redirectHandler func(w http.ResponseWriter, r *http.Request)) {
	// TODO(rmistry): CHECK FOR EDIT ACCESS!!!i
	user := srv.user(r)
	if !srv.modify.Member(user) {
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
	if err != nil {
		httputils.ReportError(w, nil, fmt.Sprintf("weeks must be an int not %s", weeks), http.StatusBadRequest)
		return
	}

	members, err := GetRotationMembers(rotationType)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to get %s rotation members.", rotationType), http.StatusInternalServerError)
		return
	}
	fmt.Println("GOT THIS!!!!!!!!!!!!!!!!!!")
	fmt.Println(members)

	// DELETE STUFF FROM THE RANGE
	rotations, err := GetUpcomingRotations(RotationTypeToKind[rotationType], from.UTC())
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to get rotations of %s.", rotationType), http.StatusInternalServerError)
		return
	}

	fmt.Println("GOT THESE ROTATIONS")
	fmt.Println(len(rotations))
	for _, rotation := range rotations {
		fmt.Println("--------------")
		fmt.Println(rotation.Username)
		fmt.Println(rotation.ScheduleStart)
		fmt.Println(rotation.ScheduleEnd)
		//fmt.Println("NOT DELETING FOR NOW")
		if err := DS.Delete(r.Context(), rotation.Key); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Could not delete rotations of %s after %s.", rotationType, from), http.StatusInternalServerError)
			return
		}
	}

	fmt.Println("----------")
	membersIndex := 0
	currScheduleFrom := from
	for week := 1; week <= weeksInt; week++ {
		currScheduleEnd := currScheduleFrom.Add(time.Hour * 24 * 6)
		if err := AddRotation(rotationType, members[membersIndex].Username, currScheduleFrom, currScheduleEnd); err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Could not create new rotations of %s.", rotationType), http.StatusInternalServerError)
			return
		}
		currScheduleFrom = currScheduleFrom.Add(time.Hour * 24 * 7)
		membersIndex = (membersIndex + 1) % len(members)
	}

	redirectHandler(w, r)
}

func (srv *Server) sheriffHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("SheriffSchedules", time.Now().UTC())
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
	if err := srv.commonHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
}

func (srv *Server) wranglerHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("GpuSheriffSchedules", time.Now().UTC())
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
	if err := srv.commonHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
}

func (srv *Server) robocopHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("RobocopSchedules", time.Now().UTC())
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
	if err := srv.commonHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
}

func (srv *Server) trooperHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("TrooperSchedules", time.Now().UTC())
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
	if err := srv.commonHandler(w, r, templateContext); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
}

func (srv *Server) commonHandler(w http.ResponseWriter, r *http.Request, templateContext RotationsTemplateContext) error {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "rotations.html", templateContext); err != nil {
		return fmt.Errorf("Failed to expand template: %s", err)
	}
	return nil
}
