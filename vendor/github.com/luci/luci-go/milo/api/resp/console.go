// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package resp

// This file contains the structures for defining a Console view.
// Console: The main entry point and the overall struct for a console page.
// BuilderRef: Used both as an input to request a builder and headers for the console.
// CommitBuild: A row in the console.  References a commit with a list of build summaries.
// ConsoleBuild: A cell in the console. Contains all information required to render the cell.

// Console represents a console view.  Commit contains the full matrix of
// Commits x Builder, and BuilderRef contains information on how to render
// the header.  The two structs are expected to be consistent.  IE len(Console.[]BuilderRef)
// Should equal len(commit.Build) for all commit in Console.Commit.
type Console struct {
	Name string

	Commit []CommitBuild

	BuilderRef []BuilderRef
}

// BuilderRef is an unambiguous reference to a builder, along with metadata on how
// to lay it out for rendering.
type BuilderRef struct {
	// Module is the name of the module this builder belongs to.  This could be "buildbot",
	// "buildbucket", or "dm".
	Module string
	// Name is the canonical reference to a specific builder in a specific module.
	Name string
	// Category is a pipe "|" deliminated list of short strings used to catagorize
	// and organize builders.  Adjacent builders with common categories will be
	// merged on the header.
	Category []string
	// ShortName is a string of length 1-3 used to label the builder.
	ShortName string
}

// CommitBuild is a row in the console.  References a commit with a list of build summaries.
type CommitBuild struct {
	Commit
	Build []*ConsoleBuild
}

// ConsoleBuild is a cell in the console. Contains all information required to render the cell.
type ConsoleBuild struct {
	// Link to the build.  Alt-text goes on the Label of the link
	Link *Link

	// Status of the build.
	Status Status
}
