// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"fmt"
	"net/http"

	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/milo/common/miloerror"
	"github.com/luci/luci-go/server/auth"
	"golang.org/x/net/context"
)

// Helper functions for ACL checking.

// IsAllowed checks to see if the user in the context is allowed to access
// the given project.
func IsAllowed(c context.Context, project string) (bool, error) {
	// Get the project, because that's where the ACLs lie.
	p, err := GetProject(c, project)
	if err != nil {
		logging.WithError(err).Errorf(c,
			"Encountered error while fetching project %s", project)
		return false, miloerror.Error{
			Message: fmt.Sprintf("Cannot fetch project %s:\n%s", project, err),
			Code:    http.StatusInternalServerError,
		}
	}

	// Alright, so who's our user?
	cu := auth.CurrentUser(c)

	for _, entry := range p.Readers {
		// Check to see if the user is listed explicitly in any of the entries.
		if cu.Email == entry {
			return true, nil
		}
		// Now check for group memberhsip.
		ok, err := auth.IsMember(c, entry)
		if err != nil {
			logging.WithError(err).Errorf(c,
				"Could not check if user is a member of %s", entry)
			return false, miloerror.Error{
				Message: fmt.Sprintf("Encountered error while checking %s:\n%s", entry, err),
				Code:    http.StatusInternalServerError,
			}

		} else if ok {
			return true, nil
		}
	}
	return false, nil
}

// IsAllowedInternal is a shorthand for checking to see if the user is a reader
// of a magic project named "buildbot-internal".
func IsAllowedInternal(c context.Context) (bool, error) {
	return IsAllowed(c, "buildbot-internal")
}
