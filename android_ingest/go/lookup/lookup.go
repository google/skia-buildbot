// Package lookup provides ...
package lookup

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/sklog"
)

// Cache keeps a cache of recent, at least four weeks, worth of buildids and
// their associated git hashes. Since this is only used for ingesting test
// results this window is fine since we can presume the tests successfully
// finish running in under a month.
type Cache struct {
	mutex sync.Mutex

	// hashes maps buildids to git hashes. We don't get fancy and use an lru
	// cache because the amount of data we store is so small. Git hashes are 40
	// chars and int64's are 8, so if we assume 100 commits to the repo per day,
	// and the application ran without restart for three years straight then this
	// data structure would grow to 48*100*365*3 bytes, which is ~5MB. Even if builds
	// arrived at the maximum of one build per second for a year we'd only use
	// 48*86400*365 bytes = 1.5 GB.
	hashes map[int64]string
}

// New returns a newly populated *Cache with buildids for the last 2 weeks.
//
// The 'checkout' is only used during the construction of *Cache.
func New(ctx context.Context, checkout *git.Checkout) (*Cache, error) {
	// Runs
	//
	//   git log main --format=oneline --since="4 weeks ago"
	//
	// to prepopulate hashes.
	log, err := checkout.Git(ctx, "log", git.MainBranch, "--format=oneline", "--since=\"2 weeks ago\"")
	if err != nil {
		return nil, fmt.Errorf("Failed to prime cache from checkout: %s", err)
	}
	c := &Cache{
		hashes: map[int64]string{},
	}
	if err := c.parseLog(log); err != nil {
		return nil, fmt.Errorf("Failed to parse git log from checkout: %s", err)
	}
	return c, nil
}

func (c *Cache) parseLog(log string) error {
	// The oneline log format looks like
	//
	//   6dab50c23b3927daf7487b4a6f105fc74aff5fa7 https://android-build.googleplex.com/builds/jump-to-build/7432561
	//   3133350e05eb07629d681c3bb61a91a51e2ff2ef https://android-build.googleplex.com/builds/jump-to-build/7432560
	//
	// if you include the commit message that poprepo adds.
	count := 0
	lines := strings.Split(log, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Split at the space between hash and url.
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return fmt.Errorf("Found invalid line: %q", line)
		}
		hash := parts[0]
		u, err := url.Parse(parts[1])
		if err != nil {
			return fmt.Errorf("Found invalid url: %q", parts[1])
		}

		// Split the URL on the slashes.
		urlParts := strings.Split(u.Path, "/")

		buildIDAsString := ""
		if len(urlParts) == 3 {
			buildIDAsString = urlParts[2]
		} else if len(urlParts) == 4 {
			buildIDAsString = urlParts[3]
		} else {
			sklog.Errorf("Found invalid url: %q", urlParts)
			continue
		}
		buildid, err := strconv.ParseInt(buildIDAsString, 10, 64)
		if err != nil {
			sklog.Errorf("Found invalid buildid: %q", urlParts[2])
			continue
		}
		c.hashes[buildid] = hash
		count++
	}
	sklog.Infof("Prepopulated lookup.Cache with %d buildids.", count)
	return nil
}

// Lookup returns the git hash for the given buildid.
func (c *Cache) Lookup(buildid int64) (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if hash, ok := c.hashes[buildid]; !ok {
		return "", fmt.Errorf("BuildId not found in cache: %d", buildid)
	} else {
		return hash, nil
	}
}

// Add a new buildid, githash to the cache.
func (c *Cache) Add(buildid int64, hash string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.hashes[buildid] = hash
}
