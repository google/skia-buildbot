package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const uncategorized = "(uncategorized)"

var (
	defaultGerritServers = []string{
		gerrit.GERRIT_SKIA_URL,
		gerrit.GERRIT_CHROMIUM_URL,
	}
	tagRegex    = regexp.MustCompile(`.*\[(.+)\].*`)
	tagSanitize = regexp.MustCompile(`[^a-zA-Z0-9-]`)

	// Flags.
	owner         = flag.String("owner", "", "Find CLs from the given owner. Defaults to the current user.")
	period        = flag.String("period", "26w", "Find CLs within the given time period.")
	gerritServers = common.NewMultiStringFlag("gerrit", nil, fmt.Sprintf("Gerrit servers to query. Can specify multiple times. Default: %+v", defaultGerritServers))

	input      = flag.String("input", "", "Don't query Gerrit; load changes from the given JSON-formatted file.")
	output     = flag.String("output", "", "Write changes to the given file. Subsequent invocations can use --input rather than querying Gerrit.")
	manualTags = common.NewMultiStringFlag("tag", nil, "Place CLs matching the given pattern in the given category: <pattern>=<category>")
)

func main() {
	common.Init()

	// Validate and interpret flags.
	if *period == "" {
		sklog.Fatal("--period is required")
	}
	dur, err := human.ParseDuration(*period)
	if err != nil {
		sklog.Fatal(err)
	}
	start := time.Now().Add(-dur)
	servers := defaultGerritServers
	if len(*gerritServers) > 0 {
		servers = *gerritServers
	}
	var manualTagRegexes map[*regexp.Regexp]string
	if manualTags != nil {
		manualTagRegexes = make(map[*regexp.Regexp]string, len(*manualTags))
		for _, arg := range *manualTags {
			split := strings.Split(arg, "=")
			if len(split) < 2 {
				sklog.Fatal("Bad format for --tag; wanted <pattern>=<category>")
			}
			// We only wanted to split on the last "=".
			category := split[len(split)-1]
			pattern := strings.Join(split[:len(split)-1], "=")
			regex, err := regexp.Compile(pattern)
			if err != nil {
				sklog.Fatal(err)
			}
			manualTagRegexes[regex] = category
		}
	}

	// Setup.
	ctx := context.Background()
	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_GERRIT)
	if err != nil {
		sklog.Fatal(err)
	}
	c := httputils.DefaultClientConfig().WithTokenSource(ts).Client()

	// Search changes.
	var changes []*gerrit.ChangeInfo
	if *input != "" {
		if err := util.WithReadFile(*input, func(r io.Reader) error {
			return json.NewDecoder(r).Decode(&changes)
		}); err != nil {
			sklog.Fatal(err)
		}
	} else {
		var changesMtx sync.Mutex
		var wg sync.WaitGroup
		for _, server := range servers {
			server := server // https://golang.org/doc/faq#closures_and_goroutines
			wg.Add(1)
			go func() {
				defer wg.Done()
				g, err := gerrit.NewGerrit(server, c)
				if err != nil {
					sklog.Fatal(err)
				}
				o := *owner
				if o == "" {
					o, err = g.GetUserEmail(ctx)
					if err != nil {
						sklog.Fatal(err)
					}
				}
				ch, err := g.Search(ctx, 9999, false, gerrit.SearchOwner(o), gerrit.SearchStatus("merged"), gerrit.SearchModifiedAfter(start))
				if err != nil {
					sklog.Fatal(err)
				}
				changesMtx.Lock()
				changes = append(changes, ch...)
				changesMtx.Unlock()
			}()
		}
		wg.Wait()
	}

	// Optionally write the changes to --output.
	if *output != "" {
		if err := util.WithWriteFile(*output, func(w io.Writer) error {
			return json.NewEncoder(w).Encode(changes)
		}); err != nil {
			sklog.Fatal(err)
		}
	}

	// Categorize changes by hashtag/topic.
	byCategory := map[string][]*gerrit.ChangeInfo{}
	for _, ch := range changes {
		categories := util.NewStringSet(ch.Hashtags)
		if ch.Topic != "" {
			categories[ch.Topic] = true
		}
		for re, cat := range manualTagRegexes {
			if re.MatchString(ch.Subject) {
				categories[cat] = true
			}
		}
		if len(categories) == 0 {
			// Try to perform our own categorization based on the
			// subject line.
			for _, match := range tagRegex.FindAllStringSubmatch(ch.Subject, -1) {
				if len(match) >= 2 {
					sanitized := tagSanitize.ReplaceAllString(match[1], "-")
					categories[sanitized] = true
				}
			}
			if len(categories) == 0 {
				categories[uncategorized] = true
			}
		}
		// Rename categories based on --tag.
		renamed := util.NewStringSet()
		for category := range categories {
			for re, cat := range manualTagRegexes {
				if re.MatchString(category) {
					category = cat
				}
			}
			renamed[category] = true
		}
		for cat := range renamed {
			byCategory[cat] = append(byCategory[cat], ch)
		}
	}
	categories := make([]string, 0, len(byCategory))
	for cat := range byCategory {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	// Report.
	fmt.Println(fmt.Sprintf("Found %d CLs", len(changes)))
	for _, cat := range categories {
		fmt.Println(fmt.Sprintf("%s: %d CLs", cat, len(byCategory[cat])))
		for _, ch := range byCategory[cat] {
			fmt.Println(fmt.Sprintf("\t%s", ch.Subject))
		}
	}
}
