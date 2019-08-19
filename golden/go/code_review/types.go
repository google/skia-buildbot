// Package code_review defines some types for getting data into and out of
// Code Review Systems (e.g. Gerrit, GitHub).
package code_review

import (
	"context"
	"errors"
	"time"
)

// The Client interface is an abstraction around a Code Review System
type Client interface {
	// GetChangeList returns the ChangeList corresponding to the given id.
	// Returns NotFound if it doesn't exist.
	GetChangeList(ctx context.Context, id string) (ChangeList, error)

	// GetPatchSets returns the PatchSets belonging to the ChangeList with the ID
	// in index order (see PatchSet.Order).
	// Returns NotFound if the ChangeList doesn't exist.
	GetPatchSets(ctx context.Context, clID string) ([]PatchSet, error)

	// GetChangeListForCommit returns the ChangeList corresponding to the given git commit.
	// Returns NotFound if it doesn't exist.
	GetChangeListForCommit(ctx context.Context, hash string) (ChangeList, error)
}

var NotFound = errors.New("not found")

type ChangeList struct {
	// SystemID is expected to be unique between all ChangeLists.
	SystemID string

	NumPatchSets int
	Owner        string
	Status       CLStatus
	Subject      string
	Updated      time.Time
}

type CLStatus int

const (
	Open CLStatus = iota
	Abandoned
	Approved
	Landed
)

type PatchSet struct {
	// SystemID may or may not be unique for all PatchSets globally.
	// Definitely unique within a given ChangeList.
	SystemID string

	ChangeListID string
	// It is convenient to think about PatchSets starting at 1 and increasing
	// monotonically. This gives some measure of time/progress. Order is the
	// index of this PatchSet relative to all other PatchSets on this CL.
	Order      int
	GitHash    string
	ParentHash string
}
