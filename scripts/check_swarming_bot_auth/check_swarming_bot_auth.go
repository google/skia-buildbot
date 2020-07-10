package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
)

var (
	pool    = flag.String("pool", "Skia", "Which Swarming pool to use.")
	server  = flag.String("server", "chromium-swarm.appspot.com", "Swarming server to use.")
	workdir = flag.String("workdir", ".", "Working directory used to find the google_storage_token.data Optional, but recommended not to use CWD.")
)

func log(f string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(f, args...))
}

func logResult(botList []*swarming_api.SwarmingRpcsBotInfo, auth string) {
	msg := fmt.Sprintf("%d\t%s", len(botList), auth)
	if len(botList) > 0 {
		msg += fmt.Sprintf("\teg. %s: %s", botList[0].BotId, botList[0].AuthenticatedAs)
	}
	log(msg)
}

func main() {
	common.Init()

	var err error
	*workdir, err = filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// Authenticated HTTP client.
	ts, err := auth.NewDefaultTokenSource(true, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	// Swarming API client.
	swarmApi, err := swarming.NewApiClient(httpClient, *server)
	if err != nil {
		sklog.Fatal(err)
	}

	// Obtain the list of bots in this pool.
	bots, err := swarmApi.ListBots(map[string]string{
		"pool": *pool,
	})
	if err != nil {
		sklog.Fatal(err)
	}
	log("%d bots in pool %s on %s.", len(bots), *pool, *server)

	// For each bot, determine whether it's using the new auth.
	var ip, bot, user, other []*swarming_api.SwarmingRpcsBotInfo
	for _, b := range bots {
		if b.AuthenticatedAs == "bot:whitelisted-ip" {
			if len(ip) == 0 {
				log("The following bots are allowed via IP:")
			}
			log("  %s", b.BotId)
			ip = append(ip, b)
		} else if strings.HasPrefix(b.AuthenticatedAs, "bot:") {
			bot = append(bot, b)
		} else if strings.HasPrefix(b.AuthenticatedAs, "user:") {
			user = append(user, b)
		} else {
			other = append(other, b)
		}
	}
	log("")
	logResult(ip, "Allowed via IP")
	logResult(bot, "as bot\t")
	logResult(user, "as user\t")
	logResult(other, "other\t")
}
