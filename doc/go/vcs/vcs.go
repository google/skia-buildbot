// Package vcs provides patching the latest revision of a CL into a local repo.
package vcs

// The command is directly done via git, a command of the form:
//
// git fetch https://skia.googlesource.com/skia refs/changes/46/4546/1 && git checkout FETCH_HEAD
//                                                            |  |   |
//                                                            |  |   +-> Patch set.
//                                                            |  |
//                                                            |  +-> Issue ID.
//                                                            |
//                                                            +-> Last two digits of Issue ID.
//
// Find the latest patchset by the largest Revision.Number.
