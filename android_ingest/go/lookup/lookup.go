// Package lookup provides ...
package lookup

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"go.skia.org/infra/go/git"
)

type Cache struct {
	mutex  sync.Mutex
	hashes map[int64]string
}

func New(checkout *git.Checkout) (*Cache, error) {
	log, err := checkout.Git("log", "master", "--format=oneline", "--since=\"4 weeks ago\"")
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
	/*
	   $ git log master --format=oneline --since="4 weeks ago"

	   6dab50c23b3927daf7487b4a6f105fc74aff5fa7 https://android-ingest.skia.org/r/3553310
	   3133350e05eb07629d681c3bb61a91a51e2ff2ef https://android-ingest.skia.org/r/3553227
	   eceadc0434451cfdce5dc6814cd48ef0f36b1dc2 https://android-ingest.skia.org/r/3553052
	   716b074f2a057324148d1af51fedd30c603da538 https://android-ingest.skia.org/r/3553049
	*/
	return nil, nil
}

func (c *Cache) parseLog(log string) error {
	lines := strings.Split(log, "\n")
	for _, line := range lines {
		parts := strings.Split(line, " ")
		if len(parts) != 2 {
			return fmt.Errorf("Found invalid line: %q", line)
		}
		hash := parts[0]
		url := parts[1]
		urlParts := strings.Split(url, "/")
		if len(urlParts) != 5 {
			return fmt.Errorf("Found invalid url: %q", url)
		}
		buildid, err := strconv.ParseInt(urlParts[4], 10, 64)
		if err != nil {
			return fmt.Errorf("Found invalid buildid: %q", urlParts[4])
		}
		c.hashes[buildid] = hash
	}
	return nil
}

func (c *Cache) Lookup(buildid int64) (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if hash, ok := c.hashes[buildid]; !ok {
		return "", fmt.Errorf("BuildId not found in cache: %d", buildid)
	} else {
		return hash, nil
	}
}

func (c *Cache) Add(buildid int64, hash string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.hashes[buildid] = hash
}
