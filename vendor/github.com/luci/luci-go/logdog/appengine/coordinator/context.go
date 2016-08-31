// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package coordinator

import (
	"fmt"

	"github.com/luci/gae/service/info"
	luciConfig "github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/grpc/grpcutil"
	"github.com/luci/luci-go/logdog/api/config/svcconfig"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
)

// NamespaceAccessType specifies the type of namespace access that is being
// requested for WithProjectNamespace.
type NamespaceAccessType int

const (
	// NamespaceAccessNoAuth grants unconditional access to a project's namespace.
	// This bypasses all ACL checks, and must only be used by service endpoints
	// that explicitly apply ACLs elsewhere.
	NamespaceAccessNoAuth NamespaceAccessType = iota

	// NamespaceAccessAllTesting is an extension of NamespaceAccessNoAuth that,
	// in addition to doing no ACL checks, also does no project existence checks.
	//
	// This must ONLY be used for testing.
	NamespaceAccessAllTesting

	// NamespaceAccessREAD enforces READ permission access to a project's
	// namespace.
	NamespaceAccessREAD

	// NamespaceAccessWRITE enforces WRITE permission access to a project's
	// namespace.
	NamespaceAccessWRITE
)

var (
	errProjectAccessDenied = grpcutil.Errf(codes.NotFound,
		"The project is invalid, or you do not have permission to access it.")
)

type servicesKeyType int

// WithServices installs the supplied Services instance into a Context.
func WithServices(c context.Context, s Services) context.Context {
	return context.WithValue(c, servicesKeyType(0), s)
}

// GetServices gets the Services instance installed in the supplied Context.
//
// If no Services has been installed, it will panic.
func GetServices(c context.Context) Services {
	s, ok := c.Value(servicesKeyType(0)).(Services)
	if !ok {
		panic("no Services instance is installed")
	}
	return s
}

// WithProjectNamespace sets the current namespace to the project name.
//
// It will return a wrapped gRPC error if the project name or the project's
// namespace is invalid.
//
// If the current user does not have the requested permission for the project, a
// MembershipError will be returned.
func WithProjectNamespace(c *context.Context, project luciConfig.ProjectName, at NamespaceAccessType) error {
	ctx := *c

	if err := project.Validate(); err != nil {
		log.WithError(err).Errorf(ctx, "Project name is invalid.")
		return grpcutil.Errf(codes.InvalidArgument, "Project name is invalid: %s", err)
	}

	// Validate the current user has the requested access.
	switch at {
	case NamespaceAccessNoAuth:
		// Assert that the project exists and has a configuration.
		if _, err := getProjectConfig(ctx, project); err != nil {
			return err
		}

	case NamespaceAccessAllTesting:
		break

	case NamespaceAccessREAD:
		pcfg, err := getProjectConfig(ctx, project)
		if err != nil {
			return err
		}

		if err := IsProjectReader(*c, pcfg); err != nil {
			log.WithError(err).Errorf(*c, "User denied READ access to requested project.")

			// Deny the existence of the project to the user.
			return errProjectAccessDenied
		}

	case NamespaceAccessWRITE:
		pcfg, err := getProjectConfig(ctx, project)
		if err != nil {
			return err
		}

		if err := IsProjectWriter(*c, pcfg); err != nil {
			log.WithError(err).Errorf(*c, "User denied WRITE access to requested project.")

			// If the user is a project reader, return PermissionDenied, since they
			// have permission to see the project. Otherwise, return NotFound to deny
			// the existence of the project.
			if err := IsProjectReader(*c, pcfg); err == nil {
				return grpcutil.Errf(codes.PermissionDenied, "User does not have WRITE access to this project.")
			}
			return errProjectAccessDenied
		}

	default:
		panic(fmt.Errorf("unknown access type: %v", at))
	}

	pns := ProjectNamespace(project)
	nc, err := info.Get(ctx).Namespace(pns)
	if err != nil {
		log.Fields{
			log.ErrorKey: err,
			"project":    project,
			"namespace":  pns,
		}.Errorf(ctx, "Failed to set namespace.")
		return grpcutil.Internal
	}

	*c = nc
	return nil
}

func getProjectConfig(ctx context.Context, project luciConfig.ProjectName) (*svcconfig.ProjectConfig, error) {
	pcfg, err := GetServices(ctx).ProjectConfig(ctx, project)
	if err != nil {
		log.WithError(err).Errorf(ctx, "Failed to load project config.")

		if err == luciConfig.ErrNoConfig {
			return nil, errProjectAccessDenied
		}
		return nil, grpcutil.Internal
	}

	return pcfg, nil
}

// Project returns the current project installed in the supplied Context's
// namespace.
//
// This function is called with the expectation that the Context is in a
// namespace conforming to ProjectNamespace. If this is not the case, this
// method will panic.
func Project(c context.Context) luciConfig.ProjectName {
	ns, _ := info.Get(c).GetNamespace()
	project := ProjectFromNamespace(ns)
	if project != "" {
		return project
	}
	panic(fmt.Errorf("current namespace %q does not begin with project namespace prefix (%q)", ns, projectNamespacePrefix))
}
