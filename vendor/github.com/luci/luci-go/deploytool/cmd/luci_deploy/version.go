// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"os/user"
	"strings"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/deploytool/api/deploy"
)

type cloudProjectVersion struct {
	// minorSourceVersion is the minorVersion parameter of the component's
	// origin Source.
	minorSourceVersion string
	// majorSourceVersion is the majorVersion parameter of the component's
	// origin Source.
	majorSourceVersion string

	// taintedUser, if not empty, indicates that this version is tainted and names
	// the user on whose behalf it was deployed.
	taintedUser string
}

func makeCloudProjectVersion(cp *layoutDeploymentCloudProject, src *layoutSource) (*cloudProjectVersion, error) {
	return (cloudProjectVersionBuilder{}).build(cp, src)
}

type cloudProjectVersionBuilder struct {
	currentUser func() (string, error)
}

func (b cloudProjectVersionBuilder) build(cp *layoutDeploymentCloudProject, src *layoutSource) (*cloudProjectVersion, error) {
	switch vs := cp.VersionScheme; vs {
	case deploy.Deployment_CloudProject_DEFAULT:
		cpv := cloudProjectVersion{
			minorSourceVersion: cloudVersionStringNormalize(src.MinorVersion),
			majorSourceVersion: cloudVersionStringNormalize(src.MajorVersion),
		}

		if src.sg.Tainted {
			username, err := b.getCurrentUser()
			if err != nil {
				return nil, errors.Annotate(err).Reason("could not get tained user").Err()
			}
			cpv.taintedUser = cloudVersionStringNormalize(username)
		}
		return &cpv, nil

	default:
		return nil, errors.Reason("unknown version scheme %(scheme)v").D("scheme", vs).Err()
	}
}

func (b *cloudProjectVersionBuilder) getCurrentUser() (string, error) {
	if b.currentUser != nil {
		return b.currentUser()
	}

	// Default "currentUser" function uses "os.User".
	user, err := user.Current()
	if err != nil {
		return "", errors.Annotate(err).Reason("could not get tained user").Err()
	}
	return user.Username, nil
}

func parseCloudProjectVersion(vs deploy.Deployment_CloudProject_VersionScheme, v string) (
	*cloudProjectVersion, error) {
	var cpv cloudProjectVersion
	switch vs {
	case deploy.Deployment_CloudProject_DEFAULT:
		parts := strings.Split(v, "-")

		switch len(parts) {
		case 4:
			// <min>-<maj>-tainted-<user>
			cpv.taintedUser = parts[3]
			fallthrough

		case 2:
			// <min>-<maj>
			cpv.minorSourceVersion = parts[0]
			cpv.majorSourceVersion = parts[1]
			return &cpv, nil

		default:
			return nil, errors.Reason("bad version %(version)q for scheme %(scheme)T").
				D("version", v).D("scheme", vs).Err()
		}

	default:
		return nil, errors.Reason("unknown version scheme %(scheme)T").D("scheme", vs).Err()
	}
}

func (v *cloudProjectVersion) String() string {
	var partsArray [5]string
	parts := partsArray[:0]

	parts = append(parts, []string{v.minorSourceVersion, v.majorSourceVersion}...)
	if tu := v.taintedUser; tu != "" {
		parts = append(parts, []string{"tainted", tu}...)
	}
	return strings.Join(parts, "-")
}

func (v *cloudProjectVersion) isTainted() bool                        { return v.taintedUser != "" }
func (v *cloudProjectVersion) Equals(other *cloudProjectVersion) bool { return *v == *other }

// cloudVersionStringNormalize converts an input string into a cloud version-
// normalized string.
//
// Any character other than [A-Za-z0-9_] is converted to an underscore.
func cloudVersionStringNormalize(v string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, v)
}
