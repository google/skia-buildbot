package main

// lsbots lists the Swarming bots that match the given dimensions or regular expressions.
//
// The following example usage reboots all skia-d-gce-* bots quarantined with
// "Failed to self-update the bot.":
//
//     $ bots=`bazel run //scripts/lsbots -- --dev --include_bot "skia-d-gce-.*" --quarantined "Failed to self-update the bot"`
//     $ for bot in $bots; do gcloud compute ssh --zone us-central1-c --project skia-swarming-bots $bot -- sudo reboot; done

import (
	"encoding/json"
	"flag"
	"fmt"
	"regexp"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

func main() {
	dev := flag.Bool("dev", false, "Run against dev swarming instance.")
	internal := flag.Bool("internal", false, "Run against internal swarming instance.")
	pool := flag.String("pool", swarming.DIMENSION_POOL_VALUE_SKIA, "Which Swarming pool to use.")
	dimensions := common.NewMultiStringFlag("dimension", nil, "Colon-separated key/value pair, eg: \"os:Linux\" dimensions of the bots to list. Can specify multiple times.")
	includeBots := common.NewMultiStringFlag("include_bot", nil, "Include these bots, regardless of whether they match the requested dimensions. Calculated AFTER the dimensions are computed. Can be simple strings or regexes.")
	excludeBots := common.NewMultiStringFlag("exclude_bot", nil, "Exclude these bots, regardless of whether they match the requested dimensions. Calculated AFTER the dimensions are computed and after --include_bot is applied. Can be simple strings or regexes.")
	quarantined := flag.String("quarantined", "", "Only include quarantined bots whose quarantine reason matches this regular expression.")

	// Setup, parse args.
	common.Init()

	if *internal && *dev {
		sklog.Fatal("Both --internal and --dev cannot be specified.")
	}

	dims, err := swarming.ParseDimensionsSingleValue(*dimensions)
	if err != nil {
		sklog.Fatalf("Problem parsing dimensions: %s", err)
	}
	dims["pool"] = *pool

	includeRegs, err := parseRegex(*includeBots)
	if err != nil {
		sklog.Fatal(err)
	}
	excludeRegs, err := parseRegex(*excludeBots)
	if err != nil {
		sklog.Fatal(err)
	}
	quarantinedRegexp, err := regexp.Compile(*quarantined)
	if err != nil {
		sklog.Fatal(err)
	}

	swarmingServer := swarming.SWARMING_SERVER
	if *internal {
		swarmingServer = swarming.SWARMING_SERVER_PRIVATE
	} else if *dev {
		swarmingServer = swarming.SWARMING_SERVER_DEV
	}

	// Authenticated HTTP client.
	ts, err := auth.NewDefaultTokenSource(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Swarming API client.
	swarmAPI, err := swarming.NewApiClient(httpClient, swarmingServer)
	if err != nil {
		sklog.Fatal(err)
	}

	// Obtain the list of bots.
	bots, err := swarmAPI.ListBots(dims)
	if err != nil {
		sklog.Fatal(err)
	}

	for _, bot := range bots {
		if len(includeRegs) > 0 && !matchesAny(bot.BotId, includeRegs) {
			sklog.Debugf("Skipping %s because it does not match --include_bot", bot.BotId)
			continue
		}

		if matchesAny(bot.BotId, excludeRegs) {
			sklog.Debugf("Skipping %s because it matches --exclude_bot", bot.BotId)
			continue
		}

		if *quarantined != "" {
			if !bot.Quarantined {
				sklog.Debugf("Skipping %s because it is not quarantined, and --quarantined was provided", bot.BotId)
				continue
			}

			state := &struct {
				Quarantined string `json:"quarantined"`
			}{}
			if err := json.Unmarshal([]byte(bot.State), &state); err != nil {
				sklog.Fatal(err)
			}
			if !quarantinedRegexp.MatchString(state.Quarantined) {
				sklog.Debugf("Skipping %s because its quarantine reason does not match --quarantined", bot.BotId)
				continue
			}
		}
		fmt.Println(bot.BotId)
	}
}

func parseRegex(flags []string) ([]*regexp.Regexp, error) {
	var regexps []*regexp.Regexp
	for _, s := range flags {
		r, err := regexp.Compile(s)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, r)
	}
	return regexps, nil
}

func matchesAny(s string, xr []*regexp.Regexp) bool {
	for _, r := range xr {
		if r.MatchString(s) {
			return true
		}
	}
	return false
}
