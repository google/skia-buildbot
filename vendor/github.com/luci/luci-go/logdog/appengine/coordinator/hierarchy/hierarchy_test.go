// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package hierarchy

import (
	"fmt"
	"testing"

	ds "github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/info"
	luciConfig "github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/logdog/appengine/coordinator"
	ct "github.com/luci/luci-go/logdog/appengine/coordinator/coordinatorTest"
	"github.com/luci/luci-go/logdog/common/types"

	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestHierarchy(t *testing.T) {
	t.Parallel()

	Convey(`With a testing configuration`, t, func() {
		c, env := ct.Install()

		var r Request
		get := func() *List {
			l, err := Get(c, r)
			if err != nil {
				panic(err)
			}
			return l
		}

		var lv listValidator

		Convey(`When requesting Project-level list`, func() {
			r.Project = ""

			Convey(`An anonymous user will see all public-access projects.`, func() {
				So(get(), lv.shouldHaveComponents, "proj-bar", "proj-foo")
			})

			Convey(`An authenticated user will see all projects.`, func() {
				env.JoinGroup("auth")

				allProjects := []interface{}{"proj-bar", "proj-exclusive", "proj-foo"}

				Convey(`Will see all projects.`, func() {
					So(get(), lv.shouldHaveComponents, allProjects...)
				})

				Convey(`Cursor and limit work.`, func() {
					r.Limit = 1

					var l *List
					for len(allProjects) > 0 {
						l = get()
						So(l, lv.shouldHaveComponents, allProjects[0])
						So(l.Next, ShouldNotEqual, "")
						allProjects = allProjects[1:]

						r.Next = l.Next
					}

					So(len(allProjects), ShouldEqual, 0)

					// End of iteration.
					l = get()
					So(l, lv.shouldHaveComponents)
					So(l.Next, ShouldEqual, "")
				})

				Convey(`Offset and limit work.`, func() {
					r.Limit = 1

					for i, proj := range allProjects {
						r.Skip = i

						l := get()
						So(l, lv.shouldHaveComponents, proj)
						So(l.Next, ShouldNotEqual, "")
					}

					// End of iteration.
					r.Skip = len(allProjects)
					l := get()
					So(l, lv.shouldHaveComponents)
					So(l.Next, ShouldEqual, "")
				})
			})
		})

		Convey(`Get within a project that the user cannot access will return NotFound.`, func() {
			r.Project = "proj-exclusive"

			_, err := Get(c, r)
			So(err, ShouldBeRPCNotFound)
		})

		Convey(`Get will return nothing when no components are registered.`, func() {
			r.Project = "proj-foo"
			lv.project = "proj-foo"

			So(get(), lv.shouldHaveComponents)

			r.PathBase = "foo"
			lv.pathBase = "foo"
			So(get(), lv.shouldHaveComponents)

			r.PathBase = "foo/+/bar"
			lv.pathBase = "foo/+/bar"
			So(get(), lv.shouldHaveComponents)
		})

		Convey(`Can register a hierarchy of name components in multiple namespaces.`, func() {
			for _, proj := range []luciConfig.ProjectName{
				"proj-foo", "proj-bar", "proj-exclusive",
			} {
				// Bypass access check.
				ic := info.Get(c).MustNamespace(coordinator.ProjectNamespace(proj))

				di := ds.Get(ic)
				for _, p := range []types.StreamPath{
					"foo/+/baz",
					"foo/+/qux",
					"foo/+/qux",
					"foo/+/qux/2468",
					"foo/+/qux/0002468",
					"foo/+/14",
					"foo/+/001337",
					"foo/+/bar",
					"foo/+/bar/baz",
					"foo/bar/+/baz",
					"bar/+/baz",
					"bar/+/baz/qux",
				} {
					comps, err := Missing(di, Components(p, true))
					if err != nil {
						panic(err)
					}

					for _, c := range comps {
						if err := c.Put(di); err != nil {
							panic(err)
						}
					}
				}
			}
			ds.Get(c).Testable().CatchupIndexes()

			Convey(`Can list the hierarchy immediate paths (discrete).`, func() {
				r.Project = "proj-foo"
				lv.project = luciConfig.ProjectName(r.Project)

				list := func(b string) *List {
					r.Project = "proj-foo"
					r.PathBase = b

					// Set up our validator for these query results.
					lv.pathBase = types.StreamPath(r.PathBase)
					return get()
				}

				So(list(""), lv.shouldHaveComponents, "bar", "foo")
				So(list("foo"), lv.shouldHaveComponents, "+", "bar")
				So(list("foo/+"), lv.shouldHaveComponents, "14$", "001337$", "bar$", "baz$", "qux$", "bar", "qux")
				So(list("foo/+/bar"), lv.shouldHaveComponents, "baz$")
				So(list("foo/bar"), lv.shouldHaveComponents, "+")
				So(list("foo/bar/+"), lv.shouldHaveComponents, "baz$")
				So(list("bar"), lv.shouldHaveComponents, "+")
				So(list("bar/+"), lv.shouldHaveComponents, "baz$", "baz")
				So(list("baz"), lv.shouldHaveComponents)
			})

			Convey(`Performing discrete queries`, func() {
				r.Project = "proj-foo"
				lv.project = luciConfig.ProjectName(r.Project)

				Convey(`When listing "proj-foo/foo/+"`, func() {
					r.PathBase = "foo/+"
					lv.pathBase = types.StreamPath(r.PathBase)

					Convey(`Can list the first 2 elements.`, func() {
						r.Limit = 2
						So(get(), lv.shouldHaveComponents, "14$", "001337$")
					})

					Convey(`Can list 3 elements, skipping the first four.`, func() {
						r.Limit = 2
						r.Skip = 4
						So(get(), lv.shouldHaveComponents, "qux$", "bar")
					})
				})

				Convey(`Can list the immediate hierarchy iteratively.`, func() {
					r.PathBase = "foo/+"
					lv.pathBase = types.StreamPath(r.PathBase)

					var all []interface{}
					for _, s := range norm(get().Comp) {
						all = append(all, s)
					}

					r.Limit = 2
					for len(all) > 0 {
						l := get()

						count := r.Limit
						if count > len(all) {
							count = len(all)
						}

						So(l, lv.shouldHaveComponents, all[:count]...)
						all = all[count:]

						if len(all) > 0 {
							So(l.Next, ShouldNotEqual, "")
						}
						r.Next = l.Next
					}
				})
			})
		})
	})
}

type listValidator struct {
	project  luciConfig.ProjectName
	pathBase types.StreamPath
}

func (lv *listValidator) shouldHaveComponents(actual interface{}, expected ...interface{}) string {
	a, ok := actual.(*List)
	if !ok {
		return fmt.Sprintf("Actual value must be a *List, not %T", actual)
	}

	// The project and path base components should match.
	if a.Project != lv.project {
		return fmt.Sprintf("Actual project %q doesn't match expected %q.", a.Project, lv.project)
	}
	if a.PathBase != lv.pathBase {
		return fmt.Sprintf("Actual path base %q doesn't match expected %q.", a.PathBase, lv.pathBase)
	}

	for i, c := range a.Comp {
		expPath := types.StreamPath(types.Construct(string(lv.pathBase), c.Name))
		if p := a.Path(c); p != expPath {
			return fmt.Sprintf("Component %d doesn't have expected path (%q != %q)", i, p, expPath)
		}
	}

	// Check normalized component values.
	comps := make([]string, len(expected))
	for i, exp := range expected {
		comps[i], ok = exp.(string)
		if !ok {
			return fmt.Sprintf("Expected values must be strings, %d is %T.", i, exp)
		}
	}
	if err := ShouldResemble(norm(a.Comp), comps); err != "" {
		return err
	}
	return ""
}

func norm(c []*ListComponent) []string {
	result := make([]string, len(c))
	for i, e := range c {
		result[i] = e.Name
		if e.Stream {
			result[i] += "$"
		}
	}
	return result
}
