package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/unrolled/secure"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
)

type Rotation struct {
	Username      string    `json:"username" datastore:"username"`
	ScheduleStart time.Time `json:"schedule_start" datastore:"schedule_start"`
	ScheduleEnd   time.Time `json:"schedule_end" datastore:"schedule_end"`

	ReadableRange string `json:"readable_range" datastore:"-"`
	CurrentWeek   bool   `json:"current_week" datastore:"-"`
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

func GetUpcomingRotations(rotationType string) ([]*Rotation, error) {
	rotations := []*Rotation{}
	q := datastore.NewQuery(rotationType).Namespace("").Filter("schedule_end >", time.Now().Local()).Order("schedule_end")
	it := DS.Run(context.TODO(), q)
	for {
		r := &Rotation{}
		_, err := it.Next(r)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of rotations: %s", err)
		}
		_, startMonth, startDate := r.ScheduleStart.Date()
		_, endMonth, endDate := r.ScheduleEnd.Date()
		r.ReadableRange = fmt.Sprintf("%d %s - %d %s", startDate, startMonth, endDate, endMonth)
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
	// CHECK FOR ERRORS HERE
	srv.updateRotationsHandler(w, r, "Sheriffs")
	srv.sheriffHandler(w, r)
}

func (srv *Server) updateRotationsHandler(w http.ResponseWriter, r *http.Request, rotationType string) {
	// TODO(rmistry): CHECK FOR EDIT ACCESS!!!i

	scheduleStart := r.URL.Query().Get("schedule_start")
	weeks := r.URL.Query().Get("weeks")
	if scheduleStart == "" || weeks == "" {
		httputils.ReportError(w, nil, "Must specify schedule_start and weeks parameters. Eg: ?schedule_start=1/31/2020&weeks=5", http.StatusBadRequest)
	}

	fmt.Println("DECODED THIS!")
	fmt.Println(scheduleStart)
	fmt.Println(weeks)

	members, err := GetRotationMembers(rotationType)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to get %s rotation members.", rotationType), http.StatusInternalServerError)
	}
	fmt.Println("GOT THIS!!!!!!!!!!!!!!!!!!")
	fmt.Println(members)

	// CREATE TIME OBJECT FROM schedule_start format.
	// DELETE STUFF FROM THE RANGE

	//templateContext := RotationsTemplateContext{
	//	Nonce:         secure.CSPNonce(r.Context()),
	//	RotationsType: "Sheriff",
	//	RotationsDoc:  "https://skia.org/dev/sheriffing",
	//	RotationsImg:  "sheriff.jpg",
	//	Rotations:     rotations,
	//}
	//if err := srv.commonHandler(w, r, templateContext); err != nil {
	//	httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
	//}
}

func (srv *Server) sheriffHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("SheriffSchedules")
	if err != nil {
		httputils.ReportError(w, err, "Failed to get sheriff rotations.", http.StatusInternalServerError)
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
	}
}

func (srv *Server) wranglerHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("GpuSheriffSchedules")
	if err != nil {
		httputils.ReportError(w, err, "Failed to get wrangler rotations.", http.StatusInternalServerError)
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
	}
}

func (srv *Server) robocopHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("RobocopSchedules")
	if err != nil {
		httputils.ReportError(w, err, "Failed to get robocop rotations.", http.StatusInternalServerError)
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
	}
}

func (srv *Server) trooperHandler(w http.ResponseWriter, r *http.Request) {
	rotations, err := GetUpcomingRotations("TrooperSchedules")
	if err != nil {
		httputils.ReportError(w, err, "Failed to get trooper rotations.", http.StatusInternalServerError)
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
