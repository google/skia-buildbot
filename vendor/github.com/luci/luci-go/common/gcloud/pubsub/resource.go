// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package pubsub

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

func newResource(project, collection, name string) string {
	return strings.Join([]string{"projects", project, collection, name}, "/")
}

func splitResource(v string) []string {
	return strings.Split(v, "/")
}

// resourceProjectName returns the resource's project and name components.
func resourceProjectName(v string) (p, n string, err error) {
	parts := splitResource(v)
	if len(parts) != 4 {
		err = errors.New("malformed resource")
		return
	}
	p, n = parts[1], parts[3]
	return
}

func resourceName(v string) (string, error) {
	parts := splitResource(v)
	if len(parts) != 4 {
		return "", errors.New("malformed resource")
	}
	return parts[3], nil
}

// validateResource validates that a resource is well-formed.
//
// A resource is in the form:
// projects/<project>/<collection>/<value>
func validateResource(v, collection string) error {
	// A resource must contain exactly three forward slashes.
	parts := splitResource(v)
	switch len(parts) {
	case 0:
		return errors.New("missing project component")
	case 1:
		return errors.New("missing project name")
	case 2:
		return errors.New("missing collection type")
	case 3:
		return errors.New("missing resource name")
	case 4:
		break
	default:
		return fmt.Errorf("too many components (%d) in resource name", len(parts))
	}

	switch {
	case parts[0] != "projects":
		return errors.New("first resource component must be 'projects'")
	case parts[2] != collection:
		return fmt.Errorf("third resource component must be '%s'", collection)
	}

	// Validate the resource name.
	if err := validateResourceName(parts[3]); err != nil {
		return err
	}
	return nil
}

// validateResourceName validates a resource name. Resource naming is described
// in: https://cloud.google.com/pubsub/overview#names
//
// As of 'v1', a resource must:
// - start with a letter.
// - end with a lowercase letter or number.
// - contain only letters, numbers, dashes (-), underscores (_) periods (.),
//   tildes (~), pluses (+), or percent signs (%).
// - be between 3 and 255 characters in length.
// - cannot begin with the string goog.
//
func validateResourceName(s string) error {
	if l := len(s); l < 3 || l > 255 {
		return fmt.Errorf("length (%d) must be between 3 and 255", l)
	}

	if strings.HasPrefix(s, "goog") {
		return errors.New("resource cannot begin with 'goog'")
	}

	// Validate correctness.
	for i, r := range s {
		if r >= unicode.MaxASCII {
			return fmt.Errorf("non-ASCII character found at index #%d", i)
		}

		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			// Must begin with an ASCII letter?
			if i == 0 {
				return errors.New("pubsub: resource names must begin with a letter")
			}

			// Is this a valid mid-resource value?
			const validMidResourceRunes = "-_.~+%"
			if !((r >= '0' && r <= '9') || strings.ContainsRune(validMidResourceRunes, r)) {
				return fmt.Errorf("pubsub: invalid resource rune at %d: %c", i, r)
			}
		}
	}
	return nil
}
