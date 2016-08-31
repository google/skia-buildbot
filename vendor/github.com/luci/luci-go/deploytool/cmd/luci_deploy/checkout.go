// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/luci/luci-go/common/cli"
	"github.com/luci/luci-go/common/errors"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/deploytool/api/deploy"
	"github.com/luci/luci-go/deploytool/managedfs"

	"github.com/maruel/subcommands"
)

// deployToolCfg is the name of the source-root deployment configuration file.
const (
	// deployToolCfg is the name of the source-root deploytool configuration file.
	deployToolCfg = "luci-deploy.cfg"

	// checkoutsSubdir is the name of the checkouts directory underneath the
	// working directory.
	checkoutsSubdir = "checkouts"
	// frozenCheckoutName is the name in the checkout directory of the frozen
	// checkout file.
	frozenCheckoutName = "checkout.frozen.cfg"

	// gitMajorVersionSize it the number of characters from the Git revision hash
	// to use for its major version.
	gitMajorVersionSize = 7
)

var cmdCheckout = subcommands.Command{
	UsageLine: "checkout",
	ShortDesc: "Performs a checkout of some or all Sources.",
	LongDesc:  "Performs a checkout of some or all Sources into the working directory.",
	CommandRun: func() subcommands.CommandRun {
		var cmd cmdCheckoutRun
		cmd.Flags.BoolVar(&cmd.local, "local", false,
			"Apply user-configured URL overrides.")
		return &cmd
	},
}

type cmdCheckoutRun struct {
	subcommands.CommandRunBase

	local bool
}

func (cmd *cmdCheckoutRun) Run(app subcommands.Application, args []string) int {
	a, c := app.(*application), cli.GetContext(app, cmd)

	// Perform the checkout.
	err := a.runWork(c, func(w *work) error {
		return checkout(w, &a.layout, cmd.local)
	})
	if err != nil {
		logError(c, err, "Failed to checkout.")
		return 1
	}
	return 0
}

func checkout(w *work, l *deployLayout, applyOverrides bool) error {
	frozen, err := l.initFrozenCheckout(w)
	if err != nil {
		return errors.Annotate(err).Reason("failed to initialize checkout").Err()
	}

	// reg is our internal checkout registry. This represents the actual
	// repository checkouts that we perform. Duplicate sources to the same
	// repository will be deduplicated here.
	fs, err := l.workingFilesystem()
	if err != nil {
		return errors.Annotate(err).Err()
	}
	checkoutDir, err := fs.Base().EnsureDirectory(checkoutsSubdir)
	if err != nil {
		return errors.Annotate(err).Reason("failed to create checkout directory").Err()
	}

	repoDir, err := checkoutDir.EnsureDirectory("repository")
	if err != nil {
		return errors.Annotate(err).Reason("failed to create repository directory %(dir)q").
			D("dir", repoDir).Err()
	}

	reg := checkoutRegistry{
		repoDir: repoDir,
	}

	// Do a central checkout of registry repositories. We will project this using
	// sorted keys so that checkout failures happen consistently.
	var (
		scs       []*sourceCheckout
		sgSources = make(map[string][]*sourceCheckout, len(frozen.SourceGroup))
	)

	sgKeys := make([]string, 0, len(frozen.SourceGroup))
	for k := range frozen.SourceGroup {
		sgKeys = append(sgKeys, k)
	}
	sort.Strings(sgKeys)

	for _, sgKey := range sgKeys {
		sg := frozen.SourceGroup[sgKey]

		srcKeys := make([]string, 0, len(sg.Source))
		groupSrcs := make([]*sourceCheckout, len(sg.Source))
		for k := range sg.Source {
			srcKeys = append(srcKeys, k)
		}
		sort.Strings(srcKeys)

		for i, srcKey := range srcKeys {
			sc := sourceCheckout{
				FrozenLayout_Source: sg.Source[srcKey],
				group:               sgKey,
				name:                srcKey,
			}

			if err := sc.addRegistryRepos(&reg); err != nil {
				return errors.Annotate(err).Reason("failed to add [%(sourceCheckout)s] to registry").
					D("sourceCheckout", sc).Err()
			}

			// If we're overriding sources, and this source is overridden, then apply
			// this and add the overriding source to the registry as well.
			//
			// We will still keep the original source in the registry so it doesn't
			// get deleted during cleanup.
			if applyOverrides {
				if override, ok := l.userSourceOverrides[sc.overrideURL]; ok {
					log.Infof(w, "Applying user repository override: [%+v] => [%+v]", sc.Source, override)

					// Any local overrides cause the source group to be considered
					// tainted.
					sc.FrozenLayout_Source.Source = override
					sc.FrozenLayout_Source.Source.Tainted = true

					if err := sc.addRegistryRepos(&reg); err != nil {
						return errors.Annotate(err).Reason("failed to add (overridden) [%(sourceCheckout)s] to registry").
							D("sourceCheckout", sc).Err()
					}
				}
			}

			groupSrcs[i] = &sc
			scs = append(scs, &sc)
		}

		sgSources[sgKey] = groupSrcs
	}
	if err := reg.checkout(w); err != nil {
		return errors.Annotate(err).Reason("failed to checkout sources").Err()
	}

	// Execute each source checkout in parallel.
	sourcesDir, err := checkoutDir.EnsureDirectory("sources")
	if err != nil {
		return errors.Annotate(err).Reason("failed to create sources directory").Err()
	}

	err = w.RunMulti(func(workC chan<- func() error) {
		for _, sc := range scs {
			sc := sc
			workC <- func() error {
				root, err := sourcesDir.EnsureDirectory(sc.group, sc.name)
				if err != nil {
					return errors.Annotate(err).Reason("failed to create checkout directory").Err()
				}

				if err := sc.checkout(w, root); err != nil {
					return errors.Annotate(err).Reason("failed to checkout %(sourceCheckout)s").
						D("sourceCheckout", sc).Err()
				}
				return nil
			}
		}
	})
	if err != nil {
		return err
	}

	// Build our source groups' checkout revision hashes.
	for _, sgKey := range sgKeys {
		sg := frozen.SourceGroup[sgKey]
		if sg.RevisionHash != "" {
			// Already calculated.
			continue
		}

		hash := sha256.New()
		for _, sc := range sgSources[sgKey] {
			if sc.Revision == "" {
				return errors.Reason("source %(sourceCheckout)q has an empty revision").
					D("sourceCheckout", sc.String()).Err()
			}
			fmt.Fprintf(hash, "%s@%s\x00", sc.name, sc.Revision)

			// If any of our sources was determined to be tainted, taint the source
			// group too.
			if sc.cs.tainted || sc.Source.Tainted {
				sg.Tainted = true
			}
		}

		sg.RevisionHash = hex.EncodeToString(hash.Sum(nil))
		log.Fields{
			"sourceGroup": sgKey,
			"revision":    sg.RevisionHash,
			"tainted":     sg.Tainted,
		}.Debugf(w, "Checked out source group.")
	}

	// Create the frozen checkout file.
	frozenFile := checkoutDir.File(frozenCheckoutName)
	if err := frozenFile.GenerateTextProto(w, frozen); err != nil {
		return errors.Annotate(err).Reason("failed to create frozen checkout protobuf").Err()
	}

	if err := checkoutDir.CleanUp(); err != nil {
		return errors.Annotate(err).Reason("failed to do full cleanup of cleanup sources filesystem").Err()
	}

	return nil
}

func checkoutFrozen(l *deployLayout) (*deploy.FrozenLayout, error) {
	path := filepath.Join(l.WorkingPath, checkoutsSubdir, frozenCheckoutName)

	var frozen deploy.FrozenLayout
	if err := unmarshalTextProtobuf(path, &frozen); err != nil {
		return nil, errors.Annotate(err).Err()
	}
	return &frozen, nil
}

// sourceCheckout manages the operation of checking out the specified layout
// source.
type sourceCheckout struct {
	*deploy.FrozenLayout_Source

	// group is the name of the source group.
	group string
	// name is the name of this source.
	name string

	// overrideURL is the calculated user config override URL that this source
	// matches.
	overrideURL string

	// cs is the checkout singleton populated from the checkout registry.
	cs *checkoutSingleton
}

func (sc *sourceCheckout) String() string {
	return fmt.Sprintf("%s.%s", sc.group, sc.name)
}

func (sc *sourceCheckout) addRegistryRepos(reg *checkoutRegistry) error {
	switch t := sc.Source.GetSource().(type) {
	case *deploy.Source_Git:
		g := t.Git

		u, err := url.Parse(g.Url)
		if err != nil {
			return errors.Annotate(err).Reason("failed to parse Git URL [%(url)s]").D("url", g.Url).Err()
		}

		// Add a Git checkout operation for this source to the registry.
		sc.cs = reg.add(&gitCheckoutOperation{
			url: u,
			ref: g.Ref,
		}, sc.Source.RunScripts)
		sc.overrideURL = g.Url
	}

	return nil
}

func (sc *sourceCheckout) checkout(w *work, root *managedfs.Dir) error {
	checkoutPath := root.File("c")

	switch t := sc.Source.GetSource().(type) {
	case *deploy.Source_Git:
		if sc.cs.path == "" {
			panic("registry repo path is not set")
		}

		// Add a symlink between our raw checkout and our current checkout.
		if err := checkoutPath.SymlinkFrom(sc.cs.path, true); err != nil {
			return errors.Annotate(err).Err()
		}
		sc.Relpath = checkoutPath.RelPath()

	default:
		return errors.Reason("don't know how to checkout %(type)T").D("type", t).Err()
	}

	sc.Revision = sc.cs.revision
	sc.MajorVersion = sc.cs.majorVersion
	sc.MinorVersion = sc.cs.minorVersion
	sc.InitResult = &deploy.SourceInitResult{
		GoPath: append(sc.Source.GoPath, sc.cs.sir.GoPath...),
	}
	return nil
}

// checkoutSingleton represents a single unique checkout.
//
// It is constructed by checkoutRegistry and populated during checkout.
type checkoutSingleton struct {
	// op is the operation to execute to populate this singleton.
	op checkoutOperation
	// runInit, if true, says that at least one of the sources is permitting
	// this repository to run its initialization scripts.
	runInit bool

	// path is the path to the base directory of the checkout. It is populated by
	// op's checkout method.
	path string
	// revision is the checkout's actual revision.
	revision string
	// majorVersion is the source's major version value.
	majorVersion string
	// minorVersion is the source's minor version value.
	minorVersion string
	// tainted is true if this checkout was detected to be tainted.
	tainted bool

	// sir is the SourceInitResult constructed during the checkout.
	sir *deploy.SourceInitResult
}

// checkoutRegistry maps unique checkout identifiers to Promise-backed checkout
// resolvers.
//
// This is a more foundational layer than "repository", and is responsible for
// actually resolving a given checkout exactly once. Multiple checkout entries
// may map to a single registry item.
type checkoutRegistry struct {
	// repoDir is the base checkout path. All checkouts will be placed
	// in hash-named files underneath of this path.
	repoDir *managedfs.Dir

	// singletons is a set of checkout singletons registered for each unique
	// repository key.
	singletons map[string]*checkoutSingleton
}

func (reg *checkoutRegistry) add(op checkoutOperation, runInit bool) *checkoutSingleton {
	key := op.key()

	cs, ok := reg.singletons[key]
	if !ok {
		cs = &checkoutSingleton{
			op: op,
		}

		if reg.singletons == nil {
			reg.singletons = make(map[string]*checkoutSingleton)
		}
		reg.singletons[key] = cs
	}
	if runInit {
		cs.runInit = true
	}

	return cs
}

func (reg *checkoutRegistry) checkout(w *work) error {
	opKeys := make([]string, 0, len(reg.singletons))
	for key := range reg.singletons {
		opKeys = append(opKeys, key)
	}
	sort.Strings(opKeys)

	err := w.RunMulti(func(workC chan<- func() error) {
		for _, key := range opKeys {
			key, cs := key, reg.singletons[key]
			workC <- func() error {
				// Generate the path of this checkout. We do this by hashing the checkout's
				// key.
				pathHash := sha256.Sum256([]byte(key))

				// Perform the actual checkout operation.
				checkoutDir, err := reg.repoDir.EnsureDirectory(hex.EncodeToString(pathHash[:]))
				if err != nil {
					return errors.Annotate(err).Reason("failed to create checkout directory for %(key)q").D("key", key).Err()
				}

				log.Fields{
					"key":          key,
					"checkoutPath": checkoutDir,
				}.Debugf(w, "Creating checkout directory.")
				if err := cs.op.checkout(w, cs, checkoutDir); err != nil {
					return err
				}

				// Make sure "checkout" did what it was supposed to.
				if cs.path == "" {
					return errors.New("checkout did not populate path")
				}

				// If there is a deployment configuration, load/parse/execute it.
				sl, err := loadSourceLayout(cs.path)
				if err != nil {
					return errors.Annotate(err).Reason("failed to load source layout").Err()
				}

				var sir deploy.SourceInitResult
				if sl != nil {
					sir.GoPath = sl.GoPath

					if len(sl.Init) > 0 {
						if cs.runInit {
							for i, in := range sl.Init {
								inResult, err := sourceInit(w, cs.path, in)
								if err != nil {
									return errors.Annotate(err).Reason("failed to run source init #%(index)d").
										D("index", i).Err()
								}

								// Merge this SourceInitResult into the common repository
								// result.
								sir.GoPath = append(sir.GoPath, inResult.GoPath...)
							}
						} else {
							log.Fields{
								"key":  key,
								"path": cs.path,
							}.Warningf(w, "Source defines initialization scripts, but is not configured to run them.")
						}
					}
				}
				cs.sir = &sir
				return nil
			}
		}
	})
	if err != nil {
		return err
	}

	return nil
}

type checkoutOperation interface {
	// key returns a unique key that describes this checkout. It should be
	// sufficiently general such that any identical checkout will share this
	// key.
	key() string

	// checkout performs the actual checkout operation.
	//
	// Upon success, checkout should populate the following checkoutSingleton
	// fields:
	//	- path
	//	- revision
	checkout(*work, *checkoutSingleton, *managedfs.Dir) error
}

type gitCheckoutOperation struct {
	// url is the URL of the Git repository.
	url *url.URL
	// ref is the Git ref to check out.
	ref string
}

func (g *gitCheckoutOperation) key() string {
	return fmt.Sprintf("git+%s@%s", g.url.String(), g.ref)
}

func (g *gitCheckoutOperation) checkout(w *work, cs *checkoutSingleton, dir *managedfs.Dir) error {
	git, err := w.git()
	if err != nil {
		return err
	}

	// If our URL is a file URL, the checkout should be an absolute symlink to the
	// file.
	var (
		path string
	)
	if g.url.Scheme == "file" {
		fileLink := dir.File("file")
		if err := fileLink.SymlinkFrom(fileURLToPath(g.url.Path), false); err != nil {
			return err
		}

		cs.tainted = true
		path = fileLink.String()
	} else {
		// This is a Git-managed directory, so we don't need to pay attention to its
		// file contents.
		dir.Ignore()

		// Get current state of target directory.
		path = dir.String()
		ref := g.ref
		if ref == "" {
			ref = "master"
		}
		gitDir := filepath.Join(path, ".git")
		needsFetch, resetRef := true, "refs/deploytool/checkout"
		switch st, err := os.Stat(gitDir); {
		case err == nil:
			if !st.IsDir() {
				return errors.Reason("checkout Git path [%(path)s] exists, and is not a directory").D("path", gitDir).Err()
			}

		case isNotExist(err):
			// If the target directory doesn't exist, run "git clone".
			log.Fields{
				"source":      g.url,
				"destination": path,
			}.Infof(w, "No current checkout; cloning...")
			if err := git.clone(w, g.url.String(), path); err != nil {
				return err
			}
			if err = git.exec(path, "update-ref", resetRef, ref).check(w); err != nil {
				return errors.Annotate(err).Reason("failed to checkout %(ref)q from %(url)q").D("ref", ref).D("url", g.url).Err()
			}
			needsFetch = false

		default:
			return errors.Annotate(err).Reason("failed to stat checkout Git directory [%(dir)s]").D("dir", gitDir).Err()
		}

		// Check out the desired commit/ref by resetting the repository.
		//
		// Check if the referenced ref is a commit that is already present in the
		// repository.
		x := git.exec(path, "rev-parse", ref)
		switch rc, err := x.run(w); {
		case err != nil:
			return errors.Annotate(err).Reason("failed to check for commit %(ref)q").D("ref", ref).Err()

		case rc == 0:
			// If the ref resolved to itself, then it's a commit and it's already in the
			// repository, so no need to fetch.
			if strings.TrimSpace(x.stdout.String()) == ref {
				resetRef = ref
				needsFetch = false
			}
			fallthrough

		default:
			// If our checkout isn't ready, fetch the ref remotely.
			if needsFetch {
				if err := git.exec(path, "fetch", "origin", fmt.Sprintf("%s:%s", ref, resetRef)).check(w); err != nil {
					return errors.Annotate(err).Reason("failed to fetch %(ref)q from remote").D("ref", ref).Err()
				}
			}

			// Reset to "resetRef".
			if err := git.exec(path, "reset", "--hard", resetRef).check(w); err != nil {
				return errors.Annotate(err).Reason("failed to checkout %(ref)q (%(localRef)q) from %(url)q").
					D("ref", ref).D("localRef", resetRef).D("url", g.url).Err()
			}
		}
	}

	// Get the current Git repository parameters.
	var (
		revision, mergeBase string
		revCount            int
	)
	err = w.RunMulti(func(workC chan<- func() error) {
		// Get HEAD revision.
		workC <- func() (err error) {
			revision, err = git.getHEAD(w, path)
			return
		}

		// Get merge base revision.
		workC <- func() (err error) {
			mergeBase, err = git.getMergeBase(w, path, "origin/master")
			return
		}

		// Get commit depth.
		workC <- func() (err error) {
			revCount, err = git.getRevListCount(w, path)
			return
		}
	})
	if err != nil {
		return errors.Annotate(err).Reason("failed to get Git repository properties").Err()
	}

	// We're tainted if our merge base doesn't equal our current revision.
	if mergeBase != revision {
		cs.tainted = true
	}

	log.Fields{
		"url":       g.url,
		"ref":       g.ref,
		"path":      path,
		"mergeBase": mergeBase,
		"revision":  revision,
		"revCount":  revCount,
		"tainted":   cs.tainted,
	}.Debugf(w, "Checked out Git repository.")

	cs.path = path
	cs.revision = revision
	cs.majorVersion = string([]rune(mergeBase)[:gitMajorVersionSize])
	cs.minorVersion = strconv.Itoa(revCount)
	return nil
}

func loadSourceLayout(path string) (*deploy.SourceLayout, error) {
	layoutPath := filepath.Join(path, deployToolCfg)
	var sl deploy.SourceLayout
	switch err := unmarshalTextProtobuf(layoutPath, &sl); {
	case err == nil:
		return &sl, nil

	case isNotExist(err):
		// There is no source layout definition in this source repository.
		return nil, nil

	default:
		// An error occurred loading the source layout.
		return nil, err
	}
}

func sourceInit(w *work, path string, in *deploy.SourceLayout_Init) (*deploy.SourceInitResult, error) {
	switch t := in.GetOperation().(type) {
	case *deploy.SourceLayout_Init_PythonScript_:
		ps := t.PythonScript

		python, err := w.python()
		if err != nil {
			return nil, err
		}

		// Create a temporary directory for the SourceInitResult.
		var r deploy.SourceInitResult
		err = withTempDir(func(tdir string) error {
			resultPath := filepath.Join(tdir, "source_init_result.cfg")

			scriptPath := deployToNative(path, ps.Path)
			if err := python.exec(scriptPath, path, resultPath).cwd(path).check(w); err != nil {
				return errors.Annotate(err).Reason("failed to execute [%(scriptPath)s]").D("scriptPath", scriptPath).Err()
			}

			switch err := unmarshalTextProtobuf(resultPath, &r); {
			case err == nil, isNotExist(err):
				return nil

			default:
				return errors.Annotate(err).Reason("failed to stat SourceInitResult [%(resultPath)s]").
					D("resultPath", resultPath).Err()
			}
		})
		return &r, err

	default:
		return nil, errors.Reason("unknown source init type %(type)T").D("type", t).Err()
	}
}
