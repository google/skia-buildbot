package main

// Gracefully shuts down groups of bots via the poorly named "terminate" api

import (
	"context"
	"flag"
	"path/filepath"
	"regexp"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2/google"
)

var (
	dimensions  = common.NewMultiStringFlag("dimension", nil, "Colon-separated key/value pair, eg: \"os:Android\" Dimensions with which to find matching tasks. Can specify multiple dimensions, bots will need to match all dimensions.")
	dryrun      = flag.Bool("dryrun", false, "Don't actually terminate the bots.")
	excludeBots = common.NewMultiStringFlag("exclude_bot", nil, "Bots which should not be affected if they match the other criteria.  Can specify multiple regexes or strings.")
	includeBots = common.NewMultiStringFlag("include_bot", nil, "Bots which will be affected, even if they don't match the other criteria.  Can specify multiple regexes or strings.")
	pool        = flag.String("pool", "", "Which Swarming pool to use.")
	verbose     = flag.Bool("verbose", false, "Display a lot of information.")
	workdir     = flag.String("workdir", ".", "Working directory used to find the google_storage_token.data Optional, but recommended not to use CWD.")
)

var (
	includeRegs []*regexp.Regexp
	excludeRegs []*regexp.Regexp
)

func main() {
	// Setup, parse args.
	common.Init()

	ctx := context.Background()

	if *pool == "" {
		sklog.Fatal("--pool is required.")
	}

	if *dimensions == nil && *includeBots == nil {
		sklog.Fatal("So one does not accidentally shutdown the entire pool, you must specify a dimension or an include rule.")
	}
	requestedDims, err := swarming.ParseDimensionsSingleValue(*dimensions)
	if err != nil {
		sklog.Fatalf("Problem parsing dimensions: %s", err)
	}
	sklog.Infof("Using dimensions: %q", requestedDims)

	includeRegs, err = parseRegex(*includeBots)
	if err != nil {
		sklog.Fatalf("Invalid regexp detected in include_bot %q: %s", *includeBots, err)
	}
	excludeRegs, err = parseRegex(*excludeBots)
	if err != nil {
		sklog.Fatalf("Invalid regexp detected in exclude_bot %q: %s", *excludeBots, err)
	}

	*workdir, err = filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Authenticated HTTP client.
	ts, err := google.DefaultTokenSource(ctx, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Swarming API client.
	swarmApi, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}

	// Obtain the list of bots in this pool.
	bots, err := swarmApi.ListBots(ctx, map[string]string{
		"pool": *pool,
	})
	if err != nil {
		sklog.Fatal(err)
	}
	logIfVerbose("%d bots in the pool", len(bots))

	matched := []string{}

	// We manually go through the list because the bot list API does not
	// support include/exclude
	for _, bot := range bots {
		if matchesAny(bot.BotId, excludeRegs) {
			logIfVerbose("Excluding %s based on exclude_bot", bot.BotId)
			continue
		}
		if matchesAny(bot.BotId, includeRegs) {
			logIfVerbose("Including %s based on include_bot", bot.BotId)
			matched = append(matched, bot.BotId)
			continue
		}
		if len(requestedDims) == 0 {
			// No specified dimensions, so only go off the include/exclude list
			continue
		}
		// Check that the bot matches all the specified dimensions.
		matchesAll := true
		for k, d := range requestedDims {
			has := false
			for _, dim := range bot.Dimensions {
				if k == dim.Key {
					has = util.In(d, dim.Value)
				}
			}
			if !has {
				matchesAll = false
				break
			}
		}
		if matchesAll {
			logIfVerbose("Including %s based on dimension(s)", bot.BotId)
			matched = append(matched, bot.BotId)
		}
	}

	if len(matched) == 0 {
		sklog.Infof("No matches.  Quitting.")
		return
	}
	sklog.Infof("The following bots will be scheduled for a graceful shutdown %q", matched)
	if *dryrun {
		sklog.Error("Exiting because of dryrun mode")
		return
	}
	if conf, err := util.AskForConfirmation("Continue?"); err != nil || !conf {
		sklog.Errorf("Not continuing (Error: %v)", err)
		return
	}

	for _, m := range matched {
		if r, err := swarmApi.GracefullyShutdownBot(ctx, m); err != nil {
			sklog.Errorf("Problem shutting down %s: %s", m, err)
		} else {
			logIfVerbose("Response from shutting down %s: %#v", m, r)
			sklog.Infof("Task for shutting down %s: https://chromium-swarm.appspot.com/task?id=%s", m, r.TaskId)
		}
	}

}

func logIfVerbose(f string, args ...interface{}) {
	if *verbose {
		sklog.Infof(f, args...)
	}
}

func parseRegex(flags []string) (retval []*regexp.Regexp, e error) {
	for _, s := range flags {
		r, err := regexp.Compile(s)
		if err != nil {
			return nil, err
		}
		retval = append(retval, r)
	}
	return retval, nil
}

func matchesAny(s string, xr []*regexp.Regexp) bool {
	for _, r := range xr {
		if r.MatchString(s) {
			return true
		}
	}
	return false
}
