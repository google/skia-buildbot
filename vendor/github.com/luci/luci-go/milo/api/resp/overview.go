// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package resp

// Collection of structs to describe how to lay out the front page of Milo.
// There is an implicit hierarchy of Master -> Builder.

type FrontPage struct {
	// Module is a Milo module, such as "buildbot", "swarmbucket", or "dm".
	Module []Module
}

// Module is a Milo module, such as "buildbot", "swarmbucket", or "dm".
type Module struct {
	// Name is the display name of the module, which could be "Buildbot",
	// "SwarmBucket", or "Dungeon Master"
	Name string

	// Masters lists all of the known masters within this module.
	Masters []MasterListing
}

// MasterListing describes all of the builders within a master cluster.
// This is also known as a "Bucket" in swarmbucket.
type MasterListing struct {
	// Name is the name of the master.
	Name string
	// Builders is a list of links to the builder page for that builder.
	Builders []Link
}
