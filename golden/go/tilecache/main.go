package main

import (
	"bytes"
	"os"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/serialize"
)

func main() {
	common.Init()

	mc := memcache.New("localhost:11211")
	mc.Timeout = 60 * time.Second

	if err := mc.Set(&memcache.Item{Key: "foo", Value: []byte("Whatever")}); err != nil {
		sklog.Fatalf("Error ping set: %s", err)
	}

	pingItem, err := mc.Get("foo")
	if err != nil {
		sklog.Fatalf("Error ping get: %s", err)
	}
	if string(pingItem.Value) != "Whatever" {
		sklog.Fatalf("")
	}

	tile, err := getTile("skia.tile")
	if err != nil {
		sklog.Fatalf("Failed to load tile: %s", err)
	}

	if err := CacheTile(mc, tile); err != nil {
		sklog.Fatalf("Failed to put tile: %s", err)
	}

	foundTile, err := GetTile(mc)
	if err != nil {
		sklog.Fatalf("Failed to get tile: %s", err)
	}

	if foundTile == nil {
		sklog.Fatal("Got nil tile")
	}
	// Compare the results to make sure they match.
}

func CacheTile(mc *memcache.Client, tile *tiling.Tile) error {
	defer timer.New("Put tile").Stop()
	var buf bytes.Buffer
	if err := serialize.SerializeTile(&buf, tile); err != nil {
		return err
	}

	return mc.Set(&memcache.Item{Key: "tile", Value: buf.Bytes()})
}

func GetTile(mc *memcache.Client) (*tiling.Tile, error) {
	defer timer.New("Get cached tile").Stop()

	it, err := mc.Get("tile")
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(it.Value)
	return serialize.DeserializeTile(buf)
}

func getTile(path string) (*tiling.Tile, error) {
	loadTimer := timer.New("Loading sample tile")
	sampledState, err := loadSample(path)
	if err != nil {
		return nil, err
	}
	loadTimer.Stop()
	return sampledState.Tile, nil
}

func loadSample(fileName string) (*serialize.Sample, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	sample, err := serialize.DeserializeSample(file)
	if err != nil {
		return nil, err
	}
	return sample, nil
}
