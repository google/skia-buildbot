package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/datastore"
)

const (
	FETCH_LIMIT = 100
)

type Rotation struct {
	Username      string    `json:"username" datastore:"username"`
	ScheduleStart time.Time `json:"schedule_start" datastore:"schedule_start"`
	ScheduleEnd   time.Time `json:"schedule_end" datastore:"schedule_end"`

	ReadableRange string `json:"readable_range" datastore:"-"`
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
	fmt.Println(time.Now())
	fmt.Println(time.Now().UTC())
	fmt.Println(time.Now().Local())
	//q := datastore.NewQuery(rotationType).Namespace("").Filter("username=", "rmistry@google.com")
	//q := datastore.NewQuery(rotationType).Namespace("").Filter("schedule_start <", time.Now())
	q := datastore.NewQuery(rotationType).Namespace("").Filter("schedule_end >", time.Now().Local()).Order("schedule_end")
	//q := datastore.NewQuery(rotationType).Namespace("").Order("schedule_end")
	it := DS.Run(context.TODO(), q)
	for {
		r := &Rotation{}
		// Combine with below
		_, err := it.Next(r)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of rotations: %s", err)
		}
		// r.ReadableRange =
		_, startMonth, startDate := r.ScheduleStart.Date()
		_, endMonth, endDate := r.ScheduleEnd.Date()
		r.ReadableRange = fmt.Sprintf("%d %s - %d %s", startDate, startMonth, endDate, endMonth)
		rotations = append(rotations, r)
	}
	for _, r := range rotations {
		fmt.Println(r.Username)
		fmt.Println(r.ReadableRange)
		//fmt.Println(r.ScheduleStart.Month())
		//fmt.Println(r.ScheduleEnd)
		fmt.Println("---------------")
	}

	fmt.Println(len(rotations))
	fmt.Println("DONE DONE DONE")
	return rotations, nil
}
