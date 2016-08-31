// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"fmt"
	"strings"

	"github.com/luci/gae/service/info"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/identity"
	"golang.org/x/net/context"
)

// IsAdminUser tests whether the current user belongs to the administrative
// users group.
//
// If the user is not, a MembershipError will be returned.
func IsAdminUser(c context.Context) error {
	cfg, err := GetServices(c).Config(c)
	if err != nil {
		return err
	}
	return checkMember(c, cfg.Coordinator.AdminAuthGroup)
}

// IsServiceUser tests whether the current user belongs to the backend services
// users group.
//
// If the user is not, a MembershipError will be returned.
func IsServiceUser(c context.Context) error {
	cfg, err := GetServices(c).Config(c)
	if err != nil {
		return err
	}
	return checkMember(c, cfg.Coordinator.ServiceAuthGroup)
}

// IsProjectReader tests whether the current user belongs to one of the
// project's declared reader groups.
//
// If the user is not, a MembershipError will be returned.
func IsProjectReader(c context.Context, pcfg *svcconfig.ProjectConfig) error {
	return checkMember(c, pcfg.ReaderAuthGroups...)
}

// IsProjectWriter tests whether the current user belongs to one of the
// project's declared writer groups.
//
// If the user is not a member of any of the groups, a MembershipError will be
// returned.
func IsProjectWriter(c context.Context, pcfg *svcconfig.ProjectConfig) error {
	return checkMember(c, pcfg.WriterAuthGroups...)
}

func checkMember(c context.Context, groups ...string) error {
	// On dev-appserver, the superuser has implicit group membership to
	// everything.
	if info.Get(c).IsDevAppServer() {
		if u := auth.CurrentUser(c); u.Superuser {
			log.Fields{
				"identity": u.Identity,
				"groups":   groups,
			}.Infof(c, "Granting superuser implicit group membership on development server.")
			return nil
		}
	}

	id := auth.CurrentIdentity(c)
	for _, group := range groups {
		is, err := auth.IsMember(c, group)
		if err != nil {
			return err
		}
		if is {
			log.Fields{
				"identity": id,
				"group":    group,
			}.Debugf(c, "User access granted.")
			return nil
		}
	}

	return &MembershipError{
		Identity: id,
		Groups:   groups,
	}
}

// MembershipError is an error returned by group membership checking functions
// if the current identity is not a member of the requested group.
type MembershipError struct {
	Identity identity.Identity
	Groups   []string
}

func (e *MembershipError) Error() string {
	return fmt.Sprintf("user %q is not a member of [%s]", e.Identity, strings.Join(e.Groups, ", "))
}

// IsMembershipError returns whether a given error is a membership error.
func IsMembershipError(e error) bool {
	_, ok := e.(*MembershipError)
	return ok
}
